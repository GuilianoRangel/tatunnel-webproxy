# Diretrizes de Implementação (AGENT.md)

Este arquivo define o fluxo de trabalho e as regras que o Agente (Antigravity) deve seguir durante a codificação do projeto **Tatunnel**.

## Regras Gerais
1. **Linguagem e Padrões:** O projeto usará Go (Golang). Siga as boas práticas, tratamento de erros robusto (sem ignorar erros críticos com `_`), e use concorrência de forma segura (goroutines, canais e `sync.Mutex`/`sync.Map` quando aplicável).
2. **Modularização:** Mantenha a lógica do túnel separada da CLI/Servidor HTTP principal para facilitar os testes.
3. **Iteração por Fases:** Implemente estritamente seguindo as etapas definidas no artefato `task.md`.
4. **Verificação Constante:** Após finalizar uma parte lógica importante (ou uma Fase), utilize o terminal (`go build`, `go test` se aplicável) para garantir que o código compila corretamente antes de prosseguir.
5. **Comunicação:** Sempre atualize o `task.md` (marcando `[x]` ou `[/]`) conforme o andamento.
6. **Logging:** Use `log` para adicionar visibilidade aos eventos (ex: cliente conectado, requisição interceptada, sessão finalizada), útil para depuração.

## Fluxo de Trabalho Esperado
1. **Etapa Atual:** Leia o `task.md` e identifique a próxima subtarefa desmarcada.
2. **Codificação:** Escreva ou modifique os arquivos usando as ferramentas de edição.
3. **Validação:** Rode comandos locais (`go run`, `go build`) para se certificar de que não há erros de sintaxe.
4. **Atualização:** Marque a tarefa como concluída no `task.md`.
5. **Relato:** Informe o progresso ao usuário.
