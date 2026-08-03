<p align="center">
  <img src="../logo.png" width="500">
</p>
<p align="center">
Deutsch / 
<a href="../README.md">English</a> / 
<a href="./README_ES.md">Español</a> / 
<a href="./README_FR.md">Français</a> / 
<a href="./README_IT.md">Italiano</a> / 
<a href="./README_PT.md">Português</a> / 
<a href="./README_RU.md">Русский</a> / 
<a href="./README_CN.md">中文</a> / 
<a href="./README_TW.md">繁體中文</a>
</p>
<hr>

Belochka (белочка, „Eichhörnchen") ist ein Server-Monitoring-Tool als einzelne Binärdatei für kleine Flotten von Linux-Servern. Es hält dauerhafte SSH-Verbindungen zu 5–20 entfernten Rechnern aufrecht und überträgt Echtzeit-Metriken für CPU, Arbeitsspeicher, Festplatten, Netzwerk und Prozesse per WebSocket an ein Browser-Dashboard. Außerdem bietet es ein interaktives Terminal im Browser für den direkten SSH-Zugriff — kein separater SSH-Client erforderlich.

## Funktionen

- **Server-Gruppen** — organisieren Sie Server in flachen Gruppen auf oberster Ebene in der Seitenleiste; Gruppen erstellen, umbenennen und löschen, Server per Drag-and-Drop oder über das Menü „Verschieben nach…" in eine Gruppe verschieben oder aus ihr entfernen; Klick auf eine Gruppe filtert das Dashboard auf ihre Mitglieder, mit Breadcrumb-Navigation
- **Echtzeit-Dashboard** — Server-Karten mit Live-Metriken für CPU, Arbeitsspeicher, Festplatte und Netzwerk, je nach Auslastung farbcodiert; Rechtsklick auf eine Karte für schnelle Aktionen Bearbeiten / Löschen / Konsole
- **Detailansicht des Servers** — CPU-Anzeigen pro Kern, Ringdiagramme für Arbeitsspeicher/Swap, Aufschlüsselung der Festplattenpartitionen, Durchsatz der Netzwerkschnittstellen
- **Prozessverwaltung** — eigener Reiter „Prozesse" mit einer flachen sortierbaren Tabelle (Spaltenbreiten anpassbar), Mehrwortsuche, Auto-Refresh-Schalter und Beenden mit SIGTERM/SIGKILL-Signalauswahl (sshd/init/systemd geschützt)
- **Web-Terminal** — vollständige interaktive SSH-Konsole im Browser über xterm.js
- **Symbol in der Systemleiste** — auf Desktop-Rechnern (Windows, macOS, Linux mit GNOME/KDE/XFCE) wird ein Tray-Symbol mit den Menüpunkten **Dashboard öffnen** und **Beenden** angezeigt; auf Servern ohne grafische Oberfläche automatischer Rückfall in den CLI-Modus
- **Authentifizierung** — Schutz durch Passwort und Session-Cookie; der erste Besuch führt durch einen zweistufigen Einrichtungsassistenten (Sprache wählen → Passwort mit Stärkeanzeige und Bestätigung festlegen); Ratenbegrenzung nach 10 fehlgeschlagenen Anmeldeversuchen (30 Minuten Sperre)
- **Einzelne Binärdatei** — Go-Backend mit eingebettetem React-Frontend; eine Datei zum Bereitstellen, nichts weiter zu installieren
- **Dauerhafte SSH-Verbindungen** — automatische Wiederverbindung mit exponentiellem Backoff und Keepalive; Verbindung vor dem Speichern eines Servers testen und beim Hinzufügen neuer Rechner den Host-Key-Fingerprint verifizieren (Trust-on-First-Use)
- **Schlüssel-Upload über den Browser** — SSH-Private-Key-Dateien direkt über die Oberfläche hochladen; Schlüssel werden validiert, unter UUID-Namen gespeichert und verwaiste Dateien automatisch bereinigt
- **Verschlüsselte Speicherung der Zugangsdaten** — Server-Passwörter werden ruhend mit AES-256-GCM verschlüsselt
- **Cron-Job-Verwaltung** — Cron-Jobs direkt auf der Serverdetailseite anzeigen, hinzufügen, bearbeiten, aktivieren/deaktivieren, löschen und ausführen
- **Batch-Befehl** — ein mehrzeiliges Skript schreiben und von der Seitenleiste aus an eine beliebige Gruppe von Servern senden; die Terminalausgabe jedes Servers wird live in den Dialog gestreamt, auf interaktive Eingabeaufforderungen antworten und den gesamten Lauf jederzeit abbrechen
- **Persistente Logdatei** — die gesamte Ausgabe wird in `belochka.log` neben der Binärdatei geschrieben (oder in das aktuelle Arbeitsverzeichnis bei Ausführung über `go run`), mit automatischer Bereinigung basierend auf der Aufbewahrungsdauer (Standard: 3 Tage)
- **Mehrsprachige Oberfläche** — Englisch, vereinfachtes Chinesisch, Französisch, Russisch, Deutsch, Spanisch, Portugiesisch, traditionelles Chinesisch und Italienisch; wählbar bei der Ersteinrichtung und umschaltbar über den Dialog „Einstellungen"; die vom Browser erkannte Sprache ist vorausgewählt
- **Einstellungen in der Anwendung** — Port, Datenverzeichnis, Sprache und Log-Aufbewahrung direkt über das Zahnrad-Symbol im Dashboard konfigurieren; keine Bearbeitung von Konfigurationsdateien erforderlich

## Schnellstart

Laden Sie die neueste Binärdatei von den [Releases](https://github.com/Unmovable8911/Belochka/releases) herunter und führen Sie sie aus:

```bash
# Linux (amd64)
chmod +x belochka-linux-amd64
./belochka-linux-amd64

# Windows (64-Bit)
belochka-windows-x86-64.exe
```

Öffnen Sie `http://localhost:53136` in Ihrem Browser. Beim ersten Besuch werden Sie aufgefordert, Ihre Sprache zu wählen und ein Passwort festzulegen — dieses schützt das Dashboard und alle API-Endpunkte. Nach der Einrichtung sind Sie automatisch angemeldet. Fügen Sie Server über die Oberfläche hinzu.

## Aus dem Quellcode erstellen

Erfordert Go 1.25+ und Node.js 18+.

```bash
git clone https://github.com/Unmovable8911/Belochka.git
cd Belochka
make build
./bin/belochka
```

Release-Binärdateien für alle Plattformen cross-kompilieren:

```bash
make release
# Ausgabe:
#   bin/belochka-linux-amd64
#   bin/belochka-linux-arm64
#   bin/belochka-windows-x86-64.exe
#   bin/belochka-windows-x86.exe
```

## Konfiguration

Belochka funktioniert ohne Konfiguration sofort einsatzbereit. Alle Einstellungen sind über den **Dialog „Einstellungen"** (Zahnrad-Symbol in der Kopfzeile des Dashboards) verfügbar. Beim ersten Start wird neben der Binärdatei automatisch eine `config.json` mit den Standardwerten erstellt. Sie können auch `--config pfad/zu/config.json` für einen benutzerdefinierten Speicherort übergeben:

```json
{
  "port": 53136,
  "data_dir": "./data",
  "language": "",
  "log_path": "",
  "log_retention_days": 3
}
```

| Feld | Standard | Beschreibung |
|---|---|---|
| `port` | `53136` | HTTP-Listen-Port |
| `data_dir` | `./data` | Datenbank, Verschlüsselungsschlüssel und hochgeladene SSH-Schlüsseldateien (`data/keys/`) |
| `language` | `""` | Sprache der Oberfläche (`en`, `zh`, `fr`, `ru`, `de`, `es`, `pt`, `zh-TW`, `it`); bei leerem Wert beim ersten Besuch automatisch erkannt |
| `log_path` | `""` | Pfad der Logdatei; falls leer, wird `belochka.log` neben der Binärdatei verwendet |
| `log_retention_days` | `3` | Anzahl der Tage, die Logeinträge aufbewahrt werden |

Änderungen an `port` und `data_dir` erfordern einen Neustart; `language` und `log_retention_days` werden über den Dialog „Einstellungen" sofort angewendet.

### Flags

| Flag | Beschreibung |
|---|---|
| `--config <pfad>` | Pfad zur JSON-Konfigurationsdatei |
| `--no-tray` | Tray-Symbol deaktivieren; als normaler CLI-Prozess ausführen |
| `--version` | Version ausgeben und beenden |

### Umgebungsvariablen

| Variable | Beschreibung |
|---|---|
| `BELOCHKA_ENCRYPTION_KEY` | AES-256-Schlüssel für die Verschlüsselung gespeicherter Passwörter; beim ersten Start automatisch generiert, falls nicht gesetzt |

### Verschlüsselungsschlüssel

Wenn beim ersten Start kein Schlüssel gesetzt ist, generiert Belochka automatisch einen unter `{data_dir}/encryption.key` und protokolliert eine Warnung. Für die Produktion setzen Sie den Schlüssel explizit über die Umgebungsvariable.
