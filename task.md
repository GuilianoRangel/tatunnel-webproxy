# Plano de Implementação - Tatunnel

Este arquivo servirá como o registro de progresso para a implementação do projeto.

- [x] **Fase 1: Configuração Inicial e Core**
  - [x] Inicializar o módulo Go (`go mod init github.com/user/tatunnel`) no diretório de workspace.
  - [x] Estruturar as pastas principais do projeto (`cmd/client`, `cmd/server`, `internal/tunnel`).
  - [x] Criar o arquivo de regras e orientações `AGENT.md`.
  - [x] Baixar as dependências base (`go get github.com/gorilla/websocket github.com/hashicorp/yamux`).

- [x] **Fase 2: O Componente Server (Relay)**
  - [x] Criar a estrutura básica do servidor web (`http.Server`).
  - [x] Criar o endpoint de handshake WebSocket (`/connect`).
  - [x] Implementar a lógica de geração de subdomínios aleatórios e validação de subdomínios customizados (verificar se já estão em uso).
  - [x] Implementar o registro em memória (`sync.Map`) mapeando "Subdomínio" para "Sessão Yamux ativa".
  - [x] Criar o proxy reverso (`http.Handler`) que intercepta requisições de domínios curinga (baseado no domínio configurado, ex: `*.tatunnel.guiliano.com.br`), extrai o subdomínio e encaminha o pacote HTTP puro pelo stream Yamux correspondente.

- [x] **Fase 3: O Componente Client (CLI)**
  - [x] Criar a estrutura da CLI do agente (`cmd/client/main.go`).
  - [x] Ler os parâmetros de linha de comando (`--url`, `--server`, `--subdomain`).
  - [x] Implementar a conexão de saída via WebSocket para o servidor.
  - [x] Enviar o handshake inicial e receber o subdomínio gerado/confirmado.
  - [x] Configurar o cliente para aceitar novos streams Yamux (atuando como um servidor TCP sobre a conexão muxada).
  - [x] Para cada stream recebido do túnel, enviar uma requisição HTTP para a URL local, ler a resposta e devolvê-la para o stream.

- [x] **Fase 4: Testes, Polimento e Deploy**
  - [x] Implementar logging estruturado para debug.
  - [x] Criar scripts/documentação para testar o sistema localmente (manipulando `/etc/hosts`).
  - [x] Criar arquivos `Dockerfile` e `docker-compose.yml` com as labels do Traefik compatíveis com o Dokploy para implantação transparente do componente Server.
  - [x] Escrever instruções de implantação e uso.
