package main

import (
	"log"
	"net/http"
	"os"

	"github.com/guiliano/tatunnel/internal/tunnel"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Porta interna do container
	}

	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tatunnel.guiliano.com.br"
	}

	srv := tunnel.NewServer(baseDomain)

	log.Printf("Tatunnel Server rodando na porta :%s (Dominio Base: %s)", port, baseDomain)
	if err := http.ListenAndServe(":"+port, srv); err != nil {
		log.Fatalf("Falha crítica no servidor: %v", err)
	}
}
