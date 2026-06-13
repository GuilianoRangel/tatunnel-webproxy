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

	localU, err := url.Parse(c.LocalURL)
	if err != nil {
		return fmt.Errorf("URL local inválida: %v", err)
	}

	log.Printf("Aguardando requisições...")

	for {
		stream, err := session.Accept()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Erro ao aceitar stream: %v", err)
			continue
		}
		go c.handleStream(stream, localU)
	}

	return nil
}

// handleStream processa cada stream vindo do servidor de forma totalmente transparente.
// Funciona para HTTP, WebSocket, SSE e qualquer protocolo — nenhum byte é modificado
// além do header Host.
func (c *Client) handleStream(stream net.Conn, localU *url.URL) {
	defer stream.Close()

	// Conecta na aplicação local via TCP puro
	localConn, err := net.Dial("tcp", localU.Host)
	if err != nil {
		log.Printf("Erro ao conectar na app local (%s): %v", localU.Host, err)
		// Tenta enviar uma resposta de erro HTTP antes de fechar
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
	defer localConn.Close()

	// Lê a requisição HTTP vinda do servidor (pelo yamux stream)
	bufReader := bufio.NewReader(stream)
	req, err := http.ReadRequest(bufReader)
	if err != nil {
		if err != io.EOF {
			log.Printf("Erro ao ler requisição do stream: %v", err)
		}
		return
	}

	// Troca o Host para apontar para a aplicação local
	req.Host = localU.Host
	req.RequestURI = ""

	// Encaminha a requisição para a aplicação local
	if err := req.Write(localConn); err != nil {
		log.Printf("Erro ao encaminhar requisição para app local: %v", err)
		return
	}

	// Copia os bytes bidirecionalmente (transparente para qualquer protocolo)
	// Direção 1: Dados restantes do servidor -> app local (ex: WebSocket frames do browser)
	// Direção 2: Resposta da app local -> servidor (ex: HTTP response, 101 Upgrade, WS frames)
	done := make(chan struct{})
	go func() {
		defer func() { done <- struct{}{} }()
		io.Copy(localConn, bufReader) // Usa bufReader para drenar buffer residual
	}()

	io.Copy(stream, localConn)
	<-done
}

