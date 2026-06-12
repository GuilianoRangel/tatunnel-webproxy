package main

import (
	"flag"
	"log"
	"os"

	"github.com/guiliano/tatunnel/internal/tunnel"
)

func main() {
	var (
		localURL  = flag.String("url", "http://localhost:8080", "A URL da sua aplicação local")
		serverURL = flag.String("server", "http://localhost:8080", "A URL do servidor Tatunnel") // Mudar para o domínio real em prod
		subdomain = flag.String("subdomain", "", "O subdomínio desejado (opcional)")
	)
	flag.Parse()

	if *localURL == "" || *serverURL == "" {
		log.Fatal("Os parâmetros --url e --server são obrigatórios.")
	}

	client := tunnel.NewClient(*serverURL, *localURL, *subdomain)

	log.Printf("Iniciando Tatunnel Client...")
	log.Printf("URL Local: %s", *localURL)
	log.Printf("Servidor: %s", *serverURL)

	err := client.Start()
	if err != nil {
		log.Printf("Túnel encerrado com erro: %v", err)
		os.Exit(1)
	}
}
