# Tatunnel

**Tatunnel** é uma ferramenta de túnel reverso simples e eficiente inspirada no `cloudflared tunnel`. 
Ela permite expor serviços locais rodando em redes restritas (atrás de NAT ou Firewalls severos) para a internet pública utilizando WebSockets e multiplexação (Yamux).

## Funcionalidades Planejadas

- **Bypass de Firewalls:** Utiliza WSS (WebSocket Secure) na porta 443, tráfego que raramente é bloqueado em redes corporativas.
- **Multiplexação Real:** Suporta múltiplas requisições HTTP simultâneas sobre uma única conexão TCP/WebSocket persistente.
- **Subdomínios Flexíveis:** Suporte para geração automática de subdomínios ou especificação de nomes customizados via `--subdomain`.
- **Deploy Containerizado:** O componente servidor está preparado para subir rapidamente usando Docker e Docker Compose.

## Como Executar (WIP)

*Em construção. Siga as tarefas de implementação detalhadas em `task.md`.*

### Setup Local Rápido (Servidor)
A arquitetura do servidor utiliza contêineres e pode ser ativada na nuvem facilmente:
```bash
docker-compose up -d
```

### Agente Local (Cliente)
```bash
tatunnel --url http://localhost:8080 --subdomain meuteste
```
