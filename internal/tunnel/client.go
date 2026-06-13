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
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

const (
	maxReconnectAttempts = 3
	baseReconnectDelay   = 2 * time.Second
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

// Start inicia a conexão com o servidor e reconecta automaticamente em caso de falha.
// Tenta reconectar até 3 vezes consecutivas. O contador reseta quando a conexão
// é bem-sucedida e começa a servir requisições.
func (c *Client) Start() error {
	attempts := 0

	for {
		wasConnected, err := c.connect()

		if err == nil {
			// Conexão encerrou normalmente (EOF limpo), sem necessidade de reconectar.
			log.Printf("Conexão encerrada normalmente.")
			return nil
		}

		// Se a conexão chegou a funcionar, reseta o contador de tentativas.
		// Isso garante que uma queda eventual após horas de uso tenha retries frescos.
		if wasConnected {
			attempts = 0
		}

		attempts++
		if attempts > maxReconnectAttempts {
			return fmt.Errorf("falha após %d tentativas de reconexão: %v", maxReconnectAttempts, err)
		}

		delay := baseReconnectDelay * time.Duration(1<<(attempts-1)) // backoff exponencial: 2s, 4s, 8s
		log.Printf("Conexão perdida: %v", err)
		log.Printf("Tentando reconexão %d/%d em %v...", attempts, maxReconnectAttempts, delay)
		time.Sleep(delay)
	}
}

// connect realiza uma única tentativa de conexão com o servidor.
// Retorna (wasConnected, err): wasConnected indica se a sessão chegou a servir
// requisições antes de cair. err é nil em caso de encerramento limpo (EOF).
func (c *Client) connect() (bool, error) {
	u, err := url.Parse(c.ServerURL)
	if err != nil {
		return false, err
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
		return false, fmt.Errorf("falha ao conectar no servidor: %v", err)
	}
	defer ws.Close()

	// Aguarda handshake de confirmação do servidor
	_, msg, err := ws.ReadMessage()
	if err != nil {
		return false, fmt.Errorf("falha no handshake: %v", err)
	}

	msgStr := string(msg)
	if strings.HasPrefix(msgStr, "ERROR:") {
		return false, fmt.Errorf("servidor recusou: %s", msgStr)
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
		return false, fmt.Errorf("falha ao iniciar yamux: %v", err)
	}
	defer session.Close()

	localU, err := url.Parse(c.LocalURL)
	if err != nil {
		return false, fmt.Errorf("URL local inválida: %v", err)
	}

	log.Printf("Aguardando requisições...")

	// Marca que a conexão está ativa — reseta o contador de reconexão
	connected := false

	for {
		stream, err := session.Accept()
		if err != nil {
			if err == io.EOF {
				// Encerramento limpo
				return connected, nil
			}
			// Erros fatais de sessão (ex: keepalive timeout) — sai do loop para reconectar
			if session.IsClosed() || strings.Contains(err.Error(), "keepalive") {
				return connected, fmt.Errorf("sessão encerrada: %v", err)
			}
			log.Printf("Erro ao aceitar stream: %v", err)
			continue
		}

		if !connected {
			connected = true
		}
		go c.handleStream(stream, localU)
	}
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

	// Preserva o Host original da requisição (ex: app.tatunnel.guiliano.com.br)
	// para que proxies internos (como o Traefik do Dokploy) consigam rotear corretamente.
	// NÃO sobrescrever req.Host com localU.Host!
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

