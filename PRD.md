# Product Requirements Document (PRD) - Tatunnel

## 1. Visão Geral
**Tatunnel** é uma ferramenta de linha de comando (CLI) e servidor relay projetada para expor aplicações web locais para a internet pública, contornando firewalls rigorosos ou a falta de IPs públicos. Ele simula o comportamento básico do `cloudflared tunnel` ou `ngrok`.

## 2. Objetivos e Proposta de Valor
* **Simplicidade:** O usuário deve conseguir expor sua aplicação com um único comando sem criar contas ou gerenciar configurações complexas (na versão inicial).
* **Bypass de Firewall:** Funcionar em ambientes corporativos onde conexões SSH de saída são bloqueadas, mas requisições HTTPS (porta 443) são permitidas.
* **Alta Performance:** Suportar requisições concorrentes sem travamentos, simulando tráfego real.

## 3. Funcionalidades Principais (MVP)
1. **Túnel via WebSocket Seguro (WSS):** A conexão principal entre o agente (Client) e o relay (Server) usará WSS na porta 443.
2. **Multiplexação:** Capacidade de processar múltiplas requisições HTTP paralelas em uma única conexão TCP/WebSocket usando o protocolo Yamux.
3. **Subdomínios Aleatórios:** Atribuição automática de um subdomínio caso o usuário não informe nenhum (ex: `a8b9xyz.tatunnel.guiliano.com.br`).
4. **Subdomínios Customizados:** Opção do usuário informar um subdomínio desejado via flag `--subdomain meunome`.
5. **Proxy Local:** O cliente encaminha o tráfego que chega do túnel para uma URL/porta local (ex: `http://localhost:8080`).
6. **Acesso Público:** Na versão inicial, as URLs geradas serão abertas para a internet, sem necessidade de autenticação.

## 4. Arquitetura e Tecnologias
* **Linguagem Principal:** Go (Golang) para Client e Server.
* **Bibliotecas Chave:**
  * `github.com/gorilla/websocket` (para o transporte WSS)
  * `github.com/hashicorp/yamux` (para multiplexação da conexão)
* **Estrutura de Deploy:**
  * **Server:** Implantação baseada em contêineres utilizando Docker Compose, integrando-se via labels do Traefik (compatível com Dokploy). O domínio base será configurável via variáveis de ambiente (ex: `tatunnel.guiliano.com.br`), permitindo que o proxy reverso da VPS repasse o tráfego sem expor as portas 80/443 diretamente.
  * **Client:** Um único executável binário na máquina do usuário.

## 5. Fluxo de Usuário
1. O usuário tem uma aplicação rodando na porta 3000 local.
2. Executa: `tatunnel --url http://localhost:3000`
3. O terminal exibe: `Túnel estabelecido! Acesse: https://h7f2k1.tatunnel.guiliano.com.br`
4. (Opcional) Executa: `tatunnel --url http://localhost:3000 --subdomain meuteste`
5. O terminal exibe: `Túnel estabelecido! Acesse: https://meuteste.tatunnel.guiliano.com.br`
6. Qualquer acesso público a essas URLs é roteado imediatamente para o `localhost:3000`.
