package tunnel

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
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
	if !strings.HasSuffix(host, s.BaseDomain) {
		http.Error(w, "Dominio Invalido", http.StatusNotFound)
		return
	}

	subdomain := strings.TrimSuffix(host, "."+s.BaseDomain)
	if subdomain == host {
		http.Error(w, "Acesse informando um subdominio", http.StatusOK)
		return
	}

	session, ok := s.Registry.Get(subdomain)
	if !ok {
		http.Error(w, "Túnel offline ou não encontrado", http.StatusNotFound)
		return
	}

	// Abre um stream dedicado sobre a conexão TCP/WS para esta requisição
	stream, err := session.Open()
	if err != nil {
		log.Printf("Falha ao abrir stream para %s: %v", subdomain, err)
		http.Error(w, "Erro no túnel", http.StatusBadGateway)
		return
	}
	defer stream.Close()

	// Clone a requisição para enviar pelo Yamux
	req := r.Clone(r.Context())
	req.RequestURI = "" // Obrigatorio limpar para o req.Write funcionar corretamente

	if err := req.Write(stream); err != nil {
		log.Printf("Falha ao escrever requisicao no stream: %v", err)
		http.Error(w, "Erro no envio", http.StatusBadGateway)
		return
	}

	// Ler a resposta que o cliente mandou de volta pelo stream
	resp, err := http.ReadResponse(bufio.NewReader(stream), req)
	if err != nil {
		log.Printf("Falha ao ler resposta do stream: %v", err)
		http.Error(w, "Erro de recepção", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copia Headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Copia Body
	io.Copy(w, resp.Body)
}
