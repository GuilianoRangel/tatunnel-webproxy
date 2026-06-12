# Plano de Implementação - Tatunnel

Este arquivo servirá como o registro de progresso para a implementação do projeto.

- [ ] **Fase 1: Configuração Inicial e Core**
  - [ ] Inicializar o módulo Go (`go mod init github.com/user/tatunnel`) no diretório de workspace.
  - [ ] Estruturar as pastas principais do projeto (`cmd/client`, `cmd/server`, `internal/tunnel`).
  - [ ] Criar o arquivo de regras e orientações `AGENT.md`.
  - [ ] Baixar as dependências base (`go get github.com/gorilla/websocket github.com/hashicorp/yamux`).

- [ ] **Fase 2: O Componente Server (Relay)**
  - [ ] Criar a estrutura básica do servidor web (`http.Server`).
  - [ ] Criar o endpoint de handshake WebSocket (`/connect`).
  - [ ] Implementar a lógica de geração de subdomínios aleatórios e validação de subdomínios customizados (verificar se já estão em uso).
  - [ ] Implementar o registro em memória (`sync.Map`) mapeando "Subdomínio" para "Sessão Yamux ativa".
  - [ ] Criar o proxy reverso (`http.Handler`) que intercepta requisições de domínios curinga (baseado no domínio configurado, ex: `*.tatunnel.guiliano.com.br`), extrai o subdomínio e encaminha o pacote HTTP puro pelo stream Yamux correspondente.

- [ ] **Fase 3: O Componente Client (CLI)**
  - [ ] Criar a estrutura da CLI do agente (`cmd/client/main.go`).
  - [ ] Ler os parâmetros de linha de comando (`--url`, `--server`, `--subdomain`).
  - [ ] Implementar a conexão de saída via WebSocket para o servidor.
  - [ ] Enviar o handshake inicial e receber o subdomínio gerado/confirmado.
  - [ ] Configurar o cliente para aceitar novos streams Yamux (atuando como um servidor TCP sobre a conexão muxada).
  - [ ] Para cada stream recebido do túnel, enviar uma requisição HTTP para a URL local, ler a resposta e devolvê-la para o stream.

- [ ] **Fase 4: Testes, Polimento e Deploy**
  - [ ] Implementar logging estruturado para debug.
  - [ ] Criar scripts/documentação para testar o sistema localmente (manipulando `/etc/hosts`).
  - [ ] Criar arquivos `Dockerfile` e `docker-compose.yml` com as labels do Traefik compatíveis com o Dokploy para implantação transparente do componente Server.
  - [ ] Escrever instruções de implantação e uso.
