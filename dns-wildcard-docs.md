# Configuração de DNS Wildcard com Cloudflare, Dokploy e Traefik

Este documento explica como configurar o domínio wildcard para o projeto usando:

* Cloudflare como provedor DNS;
* Dokploy como plataforma de deploy;
* Traefik como proxy reverso;
* Let's Encrypt com validação DNS-01;
* certificado SSL wildcard automático.

## Objetivo

Permitir que o projeto responda tanto pelo domínio principal quanto por subdomínios dinâmicos.

Exemplo:

```text
tatunnel.guiliano.com.br
teste.tatunnel.guiliano.com.br
aluno1.tatunnel.guiliano.com.br
cliente123.tatunnel.guiliano.com.br
```

Para isso, é necessário usar certificado wildcard:

```text
*.tatunnel.guiliano.com.br
```

## Por que não usar o Let's Encrypt padrão do Dokploy?

Por padrão, o Dokploy/Traefik costuma usar o desafio HTTP-01 do Let's Encrypt.

Esse método funciona bem para domínios comuns, como:

```text
app.exemplo.com.br
api.exemplo.com.br
```

Mas não funciona para certificados wildcard, como:

```text
*.tatunnel.guiliano.com.br
```

Para wildcard, o Let's Encrypt exige validação via DNS-01.

Por isso, precisamos configurar o Traefik para acessar a Cloudflare e criar automaticamente os registros DNS temporários necessários para validar o domínio.

---

# 1. Configuração do DNS na Cloudflare

Acesse o painel da Cloudflare e entre na zona DNS do domínio:

```text
guiliano.com.br
```

Crie os seguintes registros DNS:

```text
Tipo: A
Nome: tatunnel
Conteúdo: IP_DA_VPS
Proxy status: DNS only ou Proxied
```

```text
Tipo: A
Nome: *.tatunnel
Conteúdo: IP_DA_VPS
Proxy status: DNS only ou Proxied
```

Exemplo:

```text
A    tatunnel       123.123.123.123
A    *.tatunnel     123.123.123.123
```

## Recomendação inicial

Durante a configuração e testes do certificado, prefira deixar como:

```text
DNS only
```

Ou seja, nuvem cinza.

Depois que tudo estiver funcionando, você pode ativar o proxy da Cloudflare, se desejar.

---

# 2. Criar token de API na Cloudflare

O Traefik precisa de permissão para criar registros DNS temporários na Cloudflare.

Para isso, crie um token de API.

## Passos

Na Cloudflare:

1. Acesse **My Profile**;
2. Vá em **API Tokens**;
3. Clique em **Create Token**;
4. Use o template **Edit zone DNS**;
5. Configure as permissões:

```text
Zone / Zone / Read
Zone / DNS / Edit
```

6. Restrinja o token apenas à zona:

```text
guiliano.com.br
```

7. Gere o token e copie o valor.

Guarde esse token com segurança.

---

# 3. Configurar variável de ambiente no Dokploy

No painel do Dokploy, acesse:

```text
Web Server > Traefik > Environment Variables
```

Adicione a variável:

```env
CF_DNS_API_TOKEN=SEU_TOKEN_DA_CLOUDFLARE
```

Exemplo:

```env
CF_DNS_API_TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Depois salve a configuração.

---

# 4. Configurar resolver DNS-01 no Traefik

No Dokploy, acesse:

```text
Web Server > Traefik > File System
```

Edite o arquivo de configuração do Traefik, normalmente chamado:

```text
traefik.yml
```

Adicione ou ajuste a seção `certificatesResolvers`:

```yaml
certificatesResolvers:
  letsencrypt-dns:
    acme:
      email: seu-email@dominio.com
      storage: /etc/dokploy/traefik/dynamic/acme-dns.json
      dnsChallenge:
        provider: cloudflare
        resolvers:
          - "1.1.1.1:53"
          - "8.8.8.8:53"
```

Exemplo usando o domínio do projeto:

```yaml
certificatesResolvers:
  letsencrypt-dns:
    acme:
      email: guiliano@gmail.com
      storage: /etc/dokploy/traefik/dynamic/acme-dns.json
      dnsChallenge:
        provider: cloudflare
        resolvers:
          - "1.1.1.1:53"
          - "8.8.8.8:53"
```

Após salvar, reinicie o Traefik pelo Dokploy.

---

# 5. Configurar labels no docker-compose

No serviço da aplicação, configure as labels do Traefik da seguinte forma:

```yaml
labels:
  - "traefik.enable=true"

  # Domínio principal
  - "traefik.http.routers.tatunnel.rule=Host(`${BASE_DOMAIN:-tatunnel.guiliano.com.br}`)"
  - "traefik.http.routers.tatunnel.entrypoints=websecure"
  - "traefik.http.routers.tatunnel.tls=true"
  - "traefik.http.routers.tatunnel.tls.certresolver=letsencrypt"
  - "traefik.http.routers.tatunnel.tls.domains[0].main=${BASE_DOMAIN:-tatunnel.guiliano.com.br}"
  - "traefik.http.routers.tatunnel.tls.domains[0].sans=*.${BASE_DOMAIN:-tatunnel.guiliano.com.br}"

  # Subdomínios dinâmicos
  - "traefik.http.routers.tatunnel-wild.rule=HostRegexp(`{subdomain:[a-zA-Z0-9-]+}.${BASE_DOMAIN:-tatunnel.guiliano.com.br}`)"
  - "traefik.http.routers.tatunnel-wild.entrypoints=websecure"
  - "traefik.http.routers.tatunnel-wild.tls=true"
  - "traefik.http.routers.tatunnel-wild.tls.certresolver=letsencrypt"
  - "traefik.http.routers.tatunnel-wild.tls.domains[0].main=${BASE_DOMAIN:-tatunnel.guiliano.com.br}"
  - "traefik.http.routers.tatunnel-wild.tls.domains[0].sans=*.${BASE_DOMAIN:-tatunnel.guiliano.com.br}"

  # Serviço interno
  - "traefik.http.services.tatunnel.loadbalancer.server.port=8080"
```

## Sobre a variável BASE_DOMAIN

O domínio base pode ser definido via variável de ambiente:

```env
BASE_DOMAIN=tatunnel.guiliano.com.br
```

Caso essa variável não seja definida, será usado o valor padrão:

```text
tatunnel.guiliano.com.br
```

---

# 6. Exemplo completo de serviço no docker-compose

Exemplo simplificado:

```yaml
services:
  app:
    image: sua-imagem:latest
    container_name: tatunnel
    restart: unless-stopped
    environment:
      - BASE_DOMAIN=tatunnel.guiliano.com.br
    labels:
      - "traefik.enable=true"

      # Domínio principal
      - "traefik.http.routers.tatunnel.rule=Host(`${BASE_DOMAIN:-tatunnel.guiliano.com.br}`)"
      - "traefik.http.routers.tatunnel.entrypoints=websecure"
      - "traefik.http.routers.tatunnel.tls=true"
      - "traefik.http.routers.tatunnel.tls.certresolver=letsencrypt"
      - "traefik.http.routers.tatunnel.tls.domains[0].main=${BASE_DOMAIN:-tatunnel.guiliano.com.br}"
      - "traefik.http.routers.tatunnel.tls.domains[0].sans=*.${BASE_DOMAIN:-tatunnel.guiliano.com.br}"

      # Subdomínios dinâmicos
      - "traefik.http.routers.tatunnel-wild.rule=HostRegexp(`{subdomain:[a-zA-Z0-9-]+}.${BASE_DOMAIN:-tatunnel.guiliano.com.br}`)"
      - "traefik.http.routers.tatunnel-wild.entrypoints=websecure"
      - "traefik.http.routers.tatunnel-wild.tls=true"
      - "traefik.http.routers.tatunnel-wild.tls.certresolver=letsencrypt"
      - "traefik.http.routers.tatunnel-wild.tls.domains[0].main=${BASE_DOMAIN:-tatunnel.guiliano.com.br}"
      - "traefik.http.routers.tatunnel-wild.tls.domains[0].sans=*.${BASE_DOMAIN:-tatunnel.guiliano.com.br}"

      # Serviço interno
      - "traefik.http.services.tatunnel.loadbalancer.server.port=8080"
```

---

# 7. Reiniciar o serviço

Após alterar as labels, faça o redeploy da aplicação pelo Dokploy.

Também reinicie o Traefik caso tenha alterado o `traefik.yml` ou as variáveis de ambiente.

---

# 8. Verificar logs do Traefik

Para acompanhar a emissão do certificado:

```bash
docker logs -f dokploy-traefik
```

Procure mensagens relacionadas a:

```text
acme
cloudflare
dnsChallenge
certificate
letsencrypt-dns
```

Se tudo estiver correto, o Traefik irá solicitar o certificado wildcard automaticamente.

---

# 9. Testar domínio principal

Teste o domínio principal:

```bash
curl -I https://tatunnel.guiliano.com.br
```

Também é possível testar no navegador:

```text
https://tatunnel.guiliano.com.br
```

---

# 10. Testar subdomínio dinâmico

Teste qualquer subdomínio:

```bash
curl -I https://teste.tatunnel.guiliano.com.br
```

Ou acesse no navegador:

```text
https://teste.tatunnel.guiliano.com.br
```

Também podem ser usados exemplos como:

```text
https://aluno1.tatunnel.guiliano.com.br
https://cliente123.tatunnel.guiliano.com.br
```

---

# 11. Verificar certificado SSL

Para inspecionar o certificado:

```bash
openssl s_client -connect teste.tatunnel.guiliano.com.br:443 -servername teste.tatunnel.guiliano.com.br
```

O certificado deve ser válido para:

```text
tatunnel.guiliano.com.br
*.tatunnel.guiliano.com.br
```

---

# 12. Problemas comuns

## Erro: certificado não cobre o subdomínio

Verifique se as labels abaixo existem:

```yaml
- "traefik.http.routers.tatunnel-wild.tls.domains[0].main=${BASE_DOMAIN:-tatunnel.guiliano.com.br}"
- "traefik.http.routers.tatunnel-wild.tls.domains[0].sans=*.${BASE_DOMAIN:-tatunnel.guiliano.com.br}"
```

## Erro: Cloudflare API token inválido

Verifique se a variável foi configurada corretamente no Traefik:

```env
CF_DNS_API_TOKEN=SEU_TOKEN_DA_CLOUDFLARE
```

O token precisa ter permissão:

```text
Zone / DNS / Edit
Zone / Zone / Read
```

## Erro: domínio não resolve

Verifique se existem os registros DNS:

```text
A    tatunnel       IP_DA_VPS
A    *.tatunnel     IP_DA_VPS
```

## Erro: wildcard não funciona com Host

Não use:

```yaml
Host(`*.tatunnel.guiliano.com.br`)
```

Use:

```yaml
HostRegexp(`{subdomain:[a-zA-Z0-9-]+}.tatunnel.guiliano.com.br`)
```

## Erro ao emitir certificado wildcard com HTTP-01

Wildcard não funciona com HTTP-01.

Use DNS-01:

```yaml
dnsChallenge:
  provider: cloudflare
```

---

# 13. Observação sobre Cloudflare Proxy

Se o registro DNS estiver como **Proxied**, a Cloudflare também participa da camada SSL.

Para validar o certificado diretamente no Traefik, é melhor testar primeiro com:

```text
DNS only
```

Depois que estiver funcionando, o proxy da Cloudflare pode ser ativado.

---

# 14. Resumo da configuração

A configuração final usa:

```text
Cloudflare DNS
+ Cloudflare API Token
+ Traefik DNS-01 Challenge
+ Let's Encrypt
+ Certificado wildcard automático
+ Dokploy
```

Com isso, o projeto passa a aceitar automaticamente subdomínios dinâmicos em:

```text
*.tatunnel.guiliano.com.br
```

sem precisar cadastrar manualmente cada subdomínio.
