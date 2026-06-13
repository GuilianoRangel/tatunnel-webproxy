package tunnel

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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

	for {
		stream, err := session.Accept()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Erro ao aceitar stream: %v", err)
			continue
		}

		go c.handleStream(stream)
	}

	return nil
}

func (c *Client) handleStream(stream net.Conn) {
	defer stream.Close()

	req, err := http.ReadRequest(bufio.NewReader(stream))
	if err != nil {
		if err != io.EOF {
			log.Printf("Erro ao ler requisição: %v", err)
		}
		return
	}

	// Ajusta a requisição para apontar para a aplicação local
	localU, err := url.Parse(c.LocalURL)
	if err != nil {
		log.Printf("URL local inválida: %v", err)
		return
	}

	req.URL.Scheme = localU.Scheme
	req.URL.Host = localU.Host
	req.Host = localU.Host // Força o header Host ser o da aplicação local (evita erros 404 em servidores estritos)
	req.RequestURI = "" // Obrigatório limpar para o cliente HTTP do Go

	// Fazemos a chamada local sem seguir redirects automaticamente,
	// para que o redirect chegue até o browser do usuário.
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Erro ao acessar app local: %v", err)
		errorResp := &http.Response{
			StatusCode: http.StatusBadGateway,
			ProtoMajor: 1,
			ProtoMinor: 1,
			Body:       io.NopCloser(strings.NewReader("502 Bad Gateway: Aplicação local inacessível")),
			Header:     make(http.Header),
		}
		errorResp.Write(stream)
		return
	}
	defer resp.Body.Close()

	// Escreve a resposta recebida de volta para o túnel
	if err := resp.Write(stream); err != nil {
		log.Printf("Erro ao escrever resposta no stream: %v", err)
	}
}
