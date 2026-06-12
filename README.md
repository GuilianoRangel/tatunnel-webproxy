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

1. Tenha o Go instalado e compile o cliente:
   ```bash
   go build -o tatunnel ./cmd/client
   ```

2. Execute o agente, apontando a `--url` para o serviço local e `--server` para o Relay recém implantado:
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

3. **Inicie sua aplicação web:**
   ```bash
   python3 -m http.server 8080
   ```

4. **Inicie o Cliente:**
   ```bash
   go run ./cmd/client/main.go --url http://localhost:8080 --server http://localhost:8080 --subdomain meuteste
   ```

5. Acesse no navegador: `http://meuteste.tatunnel.local:8080`.
