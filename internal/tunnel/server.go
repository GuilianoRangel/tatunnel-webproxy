package tunnel

import (
	"crypto/rand"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Permite conexões de qualquer origem para o WSS do agente
	},
}

type Server struct {
	Registry   *Registry
	BaseDomain string
}

func NewServer(baseDomain string) *Server {
	return &Server{
		Registry:   NewRegistry(),
		BaseDomain: baseDomain,
	}
}

// GenerateRandomSubdomain gera uma string aleatória (ex: f8a9b2c)
func GenerateRandomSubdomain() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "tunnel"
	}
	return hex.EncodeToString(bytes)
}

// wsConnWrapper envolve a conexão websocket para implementar a interface net.Conn requerida pelo Yamux.
type wsConnWrapper struct {
	*websocket.Conn
	reader io.Reader
}

func (c *wsConnWrapper) Read(p []byte) (int, error) {
	for {
		if c.reader == nil {
			_, r, err := c.Conn.NextReader()
			if err != nil {
				return 0, err
			}
			c.reader = r
		}
		n, err := c.reader.Read(p)
		if err == io.EOF {
			c.reader = nil
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
}

func (c *wsConnWrapper) Write(p []byte) (int, error) {
	err := c.Conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// HandleConnect cuida da requisição WSS inicial vinda do Cliente/Agente
func (s *Server) HandleConnect(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Falha ao aceitar websocket: %v", err)
		return
	}
	defer ws.Close()

	requestedSubdomain := r.URL.Query().Get("subdomain")
	subdomain := requestedSubdomain

	if subdomain == "" {
		subdomain = GenerateRandomSubdomain()
	}

	conn := &wsConnWrapper{Conn: ws}

	// O Servidor age como Cliente do Yamux, pois é ele quem "Abre" os streams 
	// para mandar as requisições HTTP para o agente local (que atua como Servidor Yamux).
	session, err := yamux.Client(conn, nil)
	if err != nil {
		log.Printf("Falha ao criar sessão yamux: %v", err)
		return
	}
	defer session.Close()

	if !s.Registry.Register(subdomain, session) {
		ws.WriteMessage(websocket.TextMessage, []byte("ERROR: Subdominio indisponivel"))
		return
	}
	defer s.Registry.Unregister(subdomain)

	// Avisa o cliente que o túnel está pronto e informa o subdomínio final
	successMsg := fmt.Sprintf("CONNECTED:%s", subdomain)
	if err := ws.WriteMessage(websocket.TextMessage, []byte(successMsg)); err != nil {
		return
	}

	log.Printf("Túnel estabelecido: %s.%s", subdomain, s.BaseDomain)

	// Trava a goroutine para manter o websocket e a sessão abertos
	<-session.CloseChan()
	log.Printf("Túnel finalizado: %s.%s", subdomain, s.BaseDomain)
}

// ServeHTTP implementa o roteamento do proxy reverso
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/connect" {
		s.HandleConnect(w, r)
		return
	}

	host := r.Host
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	if !strings.HasSuffix(host, s.BaseDomain) {
		http.Error(w, "Dominio Invalido", http.StatusNotFound)
		return
	}

	subdomain := strings.TrimSuffix(host, "."+s.BaseDomain)
	if subdomain == host {
		// Se for exatamente o domínio base, serve a página de downloads e documentação
		if host == s.BaseDomain {
			http.FileServer(http.Dir("./public")).ServeHTTP(w, r)
			return
		}
		http.Error(w, "Acesse informando um subdominio", http.StatusOK)
		return
	}

	session, ok := s.Registry.Get(subdomain)
	if !ok {
		http.Error(w, "Túnel offline ou não encontrado", http.StatusNotFound)
		return
	}

	// Tratamento especial para WebSockets (Raw TCP Proxy)
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		stream, err := session.Open()
		if err != nil {
			log.Printf("Falha ao abrir stream WS para %s: %v", subdomain, err)
			http.Error(w, "Erro no túnel", http.StatusBadGateway)
			return
		}
		
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Hijack unsupported", http.StatusInternalServerError)
			return
		}
		conn, brw, err := hj.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Prepara requisição preservando o Host original e adicionando X-Forwarded
		req := r.Clone(r.Context())
		req.RequestURI = ""
		if req.Header.Get("X-Forwarded-Host") == "" {
			req.Header.Set("X-Forwarded-Host", r.Host)
		}
		if req.Header.Get("X-Forwarded-Proto") == "" {
			proto := "http"
			if r.TLS != nil {
				proto = "https"
			}
			req.Header.Set("X-Forwarded-Proto", proto)
		}
		if err := req.Write(stream); err != nil {
			conn.Close()
			stream.Close()
			return
		}

		// Copia os bytes bidirecionalmente
		go func() {
			defer conn.Close()
			defer stream.Close()
			io.Copy(stream, brw)
		}()
		go func() {
			defer conn.Close()
			defer stream.Close()
			io.Copy(conn, stream)
		}()
		return
	}

	// Utiliza ReverseProxy para requisições HTTP normais e SSE
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// Preserva o Host original do browser
			req.URL.Scheme = "http"
			req.URL.Host = "tunnel-client"
			// Adiciona headers X-Forwarded se não existirem
			if req.Header.Get("X-Forwarded-Host") == "" {
				req.Header.Set("X-Forwarded-Host", req.Host)
			}
			if req.Header.Get("X-Forwarded-Proto") == "" {
				proto := "http"
				if r.TLS != nil {
					proto = "https"
				}
				req.Header.Set("X-Forwarded-Proto", proto)
			}
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Para cada requisição, abrimos um novo stream no cliente local associado a este subdomínio
				return session.Open()
			},
			DisableKeepAlives: true, // Fecha o stream yamux após cada resposta
		},
		FlushInterval: -1, // Flush imediato para SSE (Server-Sent Events)
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Falha no proxy para %s: %v", subdomain, err)
			http.Error(w, "Erro no túnel", http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}
