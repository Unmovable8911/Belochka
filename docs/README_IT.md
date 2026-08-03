<p align="center">
  <img src="../logo.png" width="500">
</p>
<p align="center">
<a href="./README_DE.md">Deutsch</a> / 
<a href="../README.md">English</a> / 
<a href="./README_ES.md">Español</a> / 
<a href="./README_FR.md">Français</a> / 
Italiano / 
<a href="./README_PT.md">Português</a> / 
<a href="./README_RU.md">Русский</a> / 
<a href="./README_CN.md">中文</a> / 
<a href="./README_TW.md">繁體中文</a>
</p>
<hr>

Belochka (белочка, «scoiattolo») è uno strumento di monitoraggio dei server in un singolo binario per piccoli gruppi di server Linux. Mantiene connessioni SSH persistenti verso 5–20 macchine remote e trasmette metriche in tempo reale di CPU, memoria, disco, rete e processi a una dashboard nel browser tramite WebSocket. Fornisce inoltre un terminale interattivo basato sul web per l'accesso SSH diretto — nessun client SSH separato necessario.

## Funzionalità

- **Gruppi di server** — organizza i server in gruppi piatti di livello radice nella barra laterale; crea, rinomina ed elimina gruppi, e sposta i server dentro o fuori da un gruppo tramite trascinamento o il menu «Sposta in…»; clicca su un gruppo per filtrare la dashboard sui suoi membri, con navigazione a briciole di pane
- **Dashboard in tempo reale** — card dei server con metriche live di CPU, memoria, disco e rete, colorate in base all'utilizzo; clic destro su una card per azioni rapide Modifica / Elimina / Console
- **Vista dettagliata del server** — indicatori CPU per core, grafici ad anello memoria/swap, dettaglio delle partizioni disco, throughput delle interfacce di rete
- **Gestione dei processi** — scheda Processi con una tabella piatta ordinabile (larghezza colonne regolabile), ricerca multi-parola chiave, interruttore di aggiornamento automatico e terminazione con selezione del segnale SIGTERM/SIGKILL (sshd/init/systemd protetti)
- **Terminale web** — console SSH interattiva completa nel browser tramite xterm.js
- **Icona nella barra di sistema** — sui computer desktop (Windows, macOS, Linux con GNOME/KDE/XFCE), mostra un'icona nella barra con le voci **Apri dashboard** e **Esci**; passa automaticamente alla modalità CLI sui server senza interfaccia grafica
- **Autenticazione** — protezione con password + cookie di sessione; la prima visita guida attraverso una procedura di configurazione in due passaggi (scegli la lingua → imposta la password con indicatore di robustezza e conferma); limitazione del tasso dopo 10 tentativi di accesso falliti (blocco di 30 minuti)
- **Binario singolo** — backend Go con frontend React integrato; un solo file da distribuire, nient'altro da installare
- **Connessioni SSH persistenti** — riconnessione automatica con backoff esponenziale e keepalive; testa la connessione prima di salvare un server e verifica l'impronta della chiave host quando aggiungi nuove macchine (fiducia al primo utilizzo)
- **Caricamento delle chiavi dal browser** — carica file di chiave privata SSH direttamente tramite l'interfaccia; le chiavi vengono validate, salvate con nomi UUID e i file orfani vengono ripuliti automaticamente
- **Archiviazione crittografata delle credenziali** — password dei server crittografate a riposo con AES-256-GCM
- **Gestione dei job cron** — visualizza, aggiungi, modifica, abilita/disabilita, elimina ed esegui job cron direttamente dalla pagina di dettaglio del server
- **Comando batch** — scrivi uno script multi-riga e invialo a qualsiasi insieme di server dalla barra laterale; l'output del terminale di ogni server viene trasmesso in diretta nel dialogo, puoi rispondere ai prompt interattivi e annullare l'intera esecuzione in qualsiasi momento
- **File di log persistente** — tutto l'output viene scritto in `belochka.log` accanto al binario (o nella directory di lavoro corrente quando eseguito con `go run`) con pulizia automatica basata sulla conservazione (predefinito: 3 giorni)
- **Interfaccia multilingue** — inglese, cinese semplificato, francese, russo, tedesco, spagnolo, portoghese, cinese tradizionale e italiano; selezionabile durante la configurazione iniziale e modificabile dal dialogo Impostazioni; la lingua rilevata dal browser è preselezionata
- **Impostazioni integrate** — configura porta, directory dati, lingua e conservazione dei log direttamente dalla dashboard tramite l'icona a ingranaggio; nessuna modifica ai file di configurazione richiesta

## Avvio rapido

Scarica l'ultimo binario dalle [Release](https://github.com/Unmovable8911/Belochka/releases) ed eseguilo:

```bash
# Linux (amd64)
chmod +x belochka-linux-amd64
./belochka-linux-amd64

# Windows (64 bit)
belochka-windows-x86-64.exe
```

Apri `http://localhost:53136` nel browser. Alla prima visita ti verrà chiesto di scegliere la lingua e impostare una password: questa protegge la dashboard e tutti gli endpoint API. Dopo la configurazione accedi automaticamente. Aggiungi server tramite l'interfaccia.

## Compilare dai sorgenti

Richiede Go 1.25+ e Node.js 18+.

```bash
git clone https://github.com/Unmovable8911/Belochka.git
cd Belochka
make build
./bin/belochka
```

Compila in modo incrociato i binari di rilascio per tutte le piattaforme:

```bash
make release
# Output:
#   bin/belochka-linux-amd64
#   bin/belochka-linux-arm64
#   bin/belochka-windows-x86-64.exe
#   bin/belochka-windows-x86.exe
```

## Configurazione

Belochka funziona subito senza configurazione. Tutte le impostazioni sono disponibili tramite il **dialogo Impostazioni** (icona a ingranaggio nell'intestazione della dashboard). Un `config.json` con i valori predefiniti viene creato automaticamente accanto al binario al primo avvio. Puoi anche passare `--config percorso/a/config.json` per una posizione personalizzata:

```json
{
  "port": 53136,
  "data_dir": "./data",
  "language": "",
  "log_path": "",
  "log_retention_days": 3
}
```

| Campo | Predefinito | Descrizione |
|---|---|---|
| `port` | `53136` | Porta di ascolto HTTP |
| `data_dir` | `./data` | Database, chiave di crittografia e file di chiave SSH caricati (`data/keys/`) |
| `language` | `""` | Lingua dell'interfaccia (`en`, `zh`, `fr`, `ru`, `de`, `es`, `pt`, `zh-TW`, `it`); rilevata automaticamente alla prima visita se vuota |
| `log_path` | `""` | Percorso del file di log; usa `belochka.log` accanto al binario se vuoto |
| `log_retention_days` | `3` | Numero di giorni di conservazione delle voci di log |

Le modifiche a `port` e `data_dir` richiedono un riavvio; `language` e `log_retention_days` vengono applicati immediatamente tramite il dialogo Impostazioni.

### Flag

| Flag | Descrizione |
|---|---|
| `--config <percorso>` | Percorso del file di configurazione JSON |
| `--no-tray` | Disabilita l'icona nella barra di sistema; esegui come processo CLI |
| `--version` | Stampa la versione ed esci |

### Variabili d'ambiente

| Variabile | Descrizione |
|---|---|
| `BELOCHKA_ENCRYPTION_KEY` | Chiave AES-256 per le password memorizzate; generata automaticamente al primo avvio se non impostata |

### Chiave di crittografia

Al primo avvio senza una chiave impostata, Belochka ne genera automaticamente una in `{data_dir}/encryption.key` e registra un avviso. Per la produzione, imposta la chiave esplicitamente tramite la variabile d'ambiente.
