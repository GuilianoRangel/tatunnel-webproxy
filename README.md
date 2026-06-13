# Tatunnel

**Tatunnel** é uma ferramenta de túnel reverso simples e eficiente, inspirada no `cloudflared tunnel` ou `ngrok`.
Ele permite expor aplicações locais, que estão rodando em redes restritas (atrás de NAT ou Firewalls severos), para a internet pública. Isso é feito estabelecendo uma conexão de saída via WebSockets na porta 443 e multiplexando o tráfego HTTP através do protocolo Yamux.

## Componentes
- **Client (Agente):** Executado na máquina local. Estabelece uma conexão WSS segura com o servidor.
- **Server (Relay):** Hospedado em uma nuvem (VPS). Atua como proxy reverso, despachando o tráfego da web pública para o túnel correto.

---

## 🚀 Deploy do Servidor (Dokploy / VPS)

O servidor é distribuído como um container Docker e utiliza labels do Traefik para se integrar perfeitamente com o Dokploy ou outros proxies reversos.

1. Clone o repositório na sua VPS.
2. Certifique-se de ter um **domínio wildcard** configurado no seu DNS (ex: `*.tatunnel.guiliano.com.br`) apontando para a VPS.
3. Suba o container (Opcionalmente, crie um arquivo `.env` para substituir `BASE_DOMAIN`):
   ```bash
   docker-compose up -d --build
   ```

O Traefik automaticamente roteará todas as chamadas `*.tatunnel.guiliano.com.br` para a porta 8080 do container.

---

## 💻 Como usar o Cliente (Agente Local)

1. **Baixe o Agente:**
   Você não precisa compilar o cliente! Basta acessar a URL base do seu servidor (ex: `https://tatunnel.guiliano.com.br`) pelo navegador e fazer o download do executável compatível com o seu sistema (Windows, Linux ou Mac).
   
   *(Caso prefira compilar manualmente, rode `go build -o tatunnel ./cmd/client`)*

2. Execute o agente (conceda permissão com `chmod +x tatunnel` se estiver no Linux/Mac), apontando a `--url` para o serviço local e `--server` para o Relay recém implantado:
   ```bash
   ./tatunnel --url http://localhost:3000 --server https://tatunnel.guiliano.com.br
   ```

3. **Subdomínio Customizado:** Para reservar um nome específico:
   ```bash
   ./tatunnel --url http://localhost:3000 --server https://tatunnel.guiliano.com.br --subdomain meuapp
   ```

---

## 🧪 Testando Localmente (Sem VPS)

Se quiser testar a arquitetura na sua própria máquina sem depender de internet:

1. **Simule o DNS:** Edite o `/etc/hosts` da sua máquina:
   ```text
   127.0.0.1 tatunnel.local
   127.0.0.1 meuteste.tatunnel.local
   ```

2. **Inicie o Servidor:**
   ```bash
   export BASE_DOMAIN=tatunnel.local
   go run ./cmd/server/main.go
   ```

3. **Inicie sua aplicação web (em uma porta diferente):**
   ```bash
   python3 -m http.server 3000
   ```

4. **Inicie o Cliente:**
   Para testar a validação do domínio, passe a URL local simulada que você acabou de criar no `/etc/hosts`:
   ```bash
   go run ./cmd/client/main.go --url http://localhost:3000 --server http://tatunnel.local:8080 --subdomain meuteste
   ```

5. Acesse no navegador: `http://meuteste.tatunnel.local:8080`.

---

## 🛠️ Como Compilar o Projeto (Build Manual)

Caso queira modificar o código ou compilar os binários manualmente sem usar o Docker, você precisará ter o **Go 1.22+** instalado na sua máquina.

1. **Baixe as dependências:**
   ```bash
   go mod download
   ```

2. **Compilar o Agente (Client):**
   Irá gerar um executável chamado `tatunnel` na raiz do projeto.
   ```bash
   go build -o tatunnel ./cmd/client
   ```

3. **Compilar o Servidor (Relay):**
   Irá gerar um executável chamado `tatunnel-server`.
   ```bash
   go build -o tatunnel-server ./cmd/server
   ```

4. **Compilação Cruzada (Cross-Platform):**
   Você pode facilmente gerar o executável do cliente para outros sistemas operacionais. Exemplo para gerar o cliente para Windows:
   ```bash
   GOOS=windows GOARCH=amd64 go build -o tatunnel.exe ./cmd/client
   ```
