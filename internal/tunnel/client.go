package tunnel

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

// Client representa o agente que roda localmente
type Client struct {
	ServerURL string
	LocalURL  string
	Subdomain string
}

func NewClient(serverURL, localURL, subdomain string) *Client {
	return &Client{
		ServerURL: serverURL,
		LocalURL:  localURL,
		Subdomain: subdomain,
	}
}

// yamuxListener encapsula uma sessão yamux para satisfazer a interface net.Listener
type yamuxListener struct {
	*yamux.Session
}

func (l *yamuxListener) Accept() (net.Conn, error) {
	return l.Session.Accept()
}

func (l *yamuxListener) Addr() net.Addr {
	// Retorna um endereço dummy pois a conexão já está estabelecida
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
}

// Start inicia a conexão com o servidor na nuvem
func (c *Client) Start() error {
	u, err := url.Parse(c.ServerURL)
	if err != nil {
		return err
	}
	u.Path = "/connect"
	if c.Subdomain != "" {
		q := u.Query()
		q.Set("subdomain", c.Subdomain)
		u.RawQuery = q.Encode()
	}

	// Converte http -> ws, https -> wss
	wsURL := strings.Replace(u.String(), "http", "ws", 1)
	log.Printf("Conectando ao servidor: %s", wsURL)

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("falha ao conectar no servidor: %v", err)
	}
	defer ws.Close()

	// Aguarda handshake de confirmação do servidor
	_, msg, err := ws.ReadMessage()
	if err != nil {
		return fmt.Errorf("falha no handshake: %v", err)
	}

	msgStr := string(msg)
	if strings.HasPrefix(msgStr, "ERROR:") {
		return fmt.Errorf("servidor recusou: %s", msgStr)
	}

	if strings.HasPrefix(msgStr, "CONNECTED:") {
		parts := strings.Split(msgStr, ":")
		if len(parts) > 1 {
			log.Printf("Túnel estabelecido! Acesse via: %s.%s", parts[1], u.Host)
		}
	}

	// Wrapper para usar o websocket como net.Conn (já definido no server.go)
	conn := &wsConnWrapper{Conn: ws}

	// Cliente inicia o Yamux Server (ele escuta os streams abertos pelo cloud server)
	session, err := yamux.Server(conn, nil)
	if err != nil {
		return fmt.Errorf("falha ao iniciar yamux: %v", err)
	}
	defer session.Close()

	localU, err := url.Parse(c.LocalURL)
	if err != nil {
		return fmt.Errorf("URL local inválida: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(localU)

	// Garante que o Host header enviado para a aplicação local seja o LocalURL.Host original
	// Essencial para evitar 404 em aplicações que validam o Host
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = localU.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Erro ao acessar app local: %v", err)
		http.Error(w, "502 Bad Gateway: Aplicação local inacessível", http.StatusBadGateway)
	}

	// Handler customizado para interceptar WebSockets
	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			dialConn, err := net.Dial("tcp", localU.Host)
			if err != nil {
				log.Printf("Erro ao conectar no WS local: %v", err)
				http.Error(w, "App offline", http.StatusBadGateway)
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

			// Modifica e escreve a requisição
			req := r.Clone(r.Context())
			req.Host = localU.Host
			req.RequestURI = ""
			if err := req.Write(dialConn); err != nil {
				conn.Close()
				dialConn.Close()
				return
			}

			// Bidirecional
			go func() {
				defer conn.Close()
				defer dialConn.Close()
				io.Copy(dialConn, brw)
			}()
			go func() {
				defer conn.Close()
				defer dialConn.Close()
				io.Copy(conn, dialConn)
			}()
			return
		}

		// Rota normal HTTP
		proxy.ServeHTTP(w, r)
	})

	listener := &yamuxListener{Session: session}
	log.Printf("Aguardando requisições...")

	// Inicia o servidor HTTP embutido
	return http.Serve(listener, proxyHandler)
}
