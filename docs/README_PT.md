<p align="center">
  <img src="../logo.png" width="500">
</p>
<p align="center">
<a href="./README_DE.md">Deutsch</a> / 
<a href="../README.md">English</a> / 
<a href="./README_ES.md">Español</a> / 
<a href="./README_FR.md">Français</a> / 
<a href="./README_IT.md">Italiano</a> / 
Português / 
<a href="./README_RU.md">Русский</a> / 
<a href="./README_CN.md">中文</a> / 
<a href="./README_TW.md">繁體中文</a>
</p>
<hr>

Belochka (белочка, «esquilo») é uma ferramenta de monitoramento de servidores em um único binário para pequenas frotas de servidores Linux. Ela mantém conexões SSH persistentes com 5–20 máquinas remotas e transmite métricas em tempo real de CPU, memória, disco, rede e processos para um painel no navegador via WebSocket. Também oferece um terminal interativo baseado na web para acesso SSH direto — nenhum cliente SSH separado é necessário.

## Recursos

- **Grupos de servidores** — organize servidores em grupos planos de nível raiz na barra lateral; crie, renomeie e exclua grupos e mova servidores para dentro ou para fora de um grupo por arrastar e soltar ou pelo menu "Mover para…"; clique em um grupo para filtrar o painel para seus membros, com navegação por breadcrumbs
- **Painel em tempo real** — cartões de servidor com métricas ao vivo de CPU, memória, disco e rede, coloridas por nível de uso; clique com o botão direito em um cartão para ações rápidas Editar / Excluir / Console
- **Visão detalhada do servidor** — medidores de CPU por núcleo, gráficos de anel de memória/swap, detalhamento de partições de disco, throughput das interfaces de rede
- **Gerenciamento de processos** — aba Processos com uma tabela plana ordenável (largura das colunas ajustável), busca por várias palavras-chave, alternância de atualização automática e encerramento com seleção de sinal SIGTERM/SIGKILL (sshd/init/systemd protegidos)
- **Terminal web** — console SSH interativo completo no navegador via xterm.js
- **Ícone na bandeja do sistema** — em máquinas de desktop (Windows, macOS, Linux com GNOME/KDE/XFCE), mostra um ícone na bandeja com os itens **Abrir painel** e **Sair**; muda automaticamente para o modo CLI em servidores sem interface gráfica
- **Autenticação** — proteção por senha + cookie de sessão; a primeira visita passa por um assistente de configuração em duas etapas (escolher idioma → definir senha com medidor de força e confirmação); limite de taxa após 10 tentativas de login falhas (bloqueio de 30 minutos)
- **Binário único** — backend Go com frontend React incorporado; um único arquivo para implantar, nada mais para instalar
- **Conexões SSH persistentes** — reconexão automática com backoff exponencial e keepalive; teste a conexão antes de salvar um servidor e verifique a impressão digital da chave do host ao adicionar novas máquinas (confiança no primeiro uso)
- **Upload de chaves pelo navegador** — envie arquivos de chave privada SSH diretamente pela interface; as chaves são validadas, armazenadas com nomes UUID e arquivos órfãos são limpos automaticamente
- **Armazenamento criptografado de credenciais** — senhas de servidores criptografadas em repouso com AES-256-GCM
- **Gerenciamento de tarefas cron** — ver, adicionar, editar, habilitar/desabilitar, excluir e executar tarefas cron diretamente na página de detalhes do servidor
- **Comando em lote** — escreva um script de várias linhas e envie-o para qualquer conjunto de servidores pela barra lateral; a saída do terminal de cada servidor é transmitida ao vivo para o diálogo, você pode responder a prompts interativos e cancelar toda a execução a qualquer momento
- **Arquivo de log persistente** — toda a saída é gravada em `belochka.log` ao lado do binário (ou no diretório de trabalho atual quando executado via `go run`) com limpeza automática por retenção (padrão: 3 dias)
- **Interface multilíngue** — inglês, chinês simplificado, francês, russo, alemão, espanhol, português, chinês tradicional e italiano; selecionável durante a configuração inicial e alternável pelo diálogo de Configurações; o idioma detectado pelo navegador é pré-selecionado
- **Configurações no aplicativo** — configure porta, diretório de dados, idioma e retenção de logs diretamente do painel pelo ícone de engrenagem; sem necessidade de editar arquivos de configuração

## Início rápido

Baixe o binário mais recente das [Releases](https://github.com/Unmovable8911/Belochka/releases) e execute-o:

```bash
# Linux (amd64)
chmod +x belochka-linux-amd64
./belochka-linux-amd64

# Windows (64 bits)
belochka-windows-x86-64.exe
```

Abra `http://localhost:53136` no seu navegador. Na primeira visita, você será solicitado a escolher o idioma e definir uma senha — isso protege o painel e todos os endpoints da API. Após a configuração, você faz login automaticamente. Adicione servidores pela interface.

## Compilar a partir do código-fonte

Requer Go 1.25+ e Node.js 18+.

```bash
git clone https://github.com/Unmovable8911/Belochka.git
cd Belochka
make build
./bin/belochka
```

Compile cruzadamente os binários de lançamento para todas as plataformas:

```bash
make release
# Saída:
#   bin/belochka-linux-amd64
#   bin/belochka-linux-arm64
#   bin/belochka-windows-x86-64.exe
#   bin/belochka-windows-x86.exe
```

## Configuração

O Belochka funciona prontamente sem configuração. Todas as configurações estão disponíveis no **diálogo de Configurações** (ícone de engrenagem no cabeçalho do painel). Um `config.json` com valores padrão é criado automaticamente ao lado do binário no primeiro início. Você também pode passar `--config caminho/para/config.json` para um local personalizado:

```json
{
  "port": 53136,
  "data_dir": "./data",
  "language": "",
  "log_path": "",
  "log_retention_days": 3
}
```

| Campo | Padrão | Descrição |
|---|---|---|
| `port` | `53136` | Porta de escuta HTTP |
| `data_dir` | `./data` | Banco de dados, chave de criptografia e arquivos de chave SSH enviados (`data/keys/`) |
| `language` | `""` | Idioma da interface (`en`, `zh`, `fr`, `ru`, `de`, `es`, `pt`, `zh-TW`, `it`); detectado automaticamente na primeira visita se vazio |
| `log_path` | `""` | Caminho do arquivo de log; usa `belochka.log` ao lado do binário se vazio |
| `log_retention_days` | `3` | Número de dias para manter as entradas de log |

Alterações em `port` e `data_dir` exigem reinicialização; `language` e `log_retention_days` são aplicados imediatamente pelo diálogo de Configurações.

### Flags

| Flag | Descrição |
|---|---|
| `--config <caminho>` | Caminho para o arquivo de configuração JSON |
| `--no-tray` | Desabilitar o ícone da bandeja; executar como processo CLI |
| `--version` | Imprimir a versão e sair |

### Variáveis de ambiente

| Variável | Descrição |
|---|---|
| `BELOCHKA_ENCRYPTION_KEY` | Chave AES-256 para senhas armazenadas; gerada automaticamente no primeiro início se não estiver definida |

### Chave de criptografia

No primeiro início sem uma chave definida, o Belochka gera automaticamente uma em `{data_dir}/encryption.key` e registra um aviso. Para produção, defina a chave explicitamente via variável de ambiente.
