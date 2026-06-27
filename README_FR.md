<p align="center">
  <img src="./logo.png" width="500">
</p>
<p align="center">
<a href="./README.md">English</a> / 
<a href="./README_CN.md">中文</a> / 
Français / 
<a href="./README_RU.md">Русский</a>
</p>
<hr>

Belochka (белочка, « écureuil ») est un outil de surveillance serveur mono-binaire conçu pour de petites flottes de serveurs Linux. Il maintient des connexions SSH persistantes vers 5 à 20 machines distantes et diffuse en temps réel les métriques CPU, mémoire, disque, réseau et processus vers un tableau de bord navigateur via WebSocket. Il fournit également un terminal interactif web pour un accès SSH direct — aucun client SSH séparé requis.

## Fonctionnalités

- **Tableau de bord en temps réel** — cartes serveur avec métriques CPU, mémoire, disque et réseau en direct, colorées selon l'utilisation
- **Vue détaillée du serveur** — jauges CPU par cœur, graphiques en anneau mémoire/swap, détail des partitions disque, débit par interface réseau, tableau de processus triable
- **Terminal web** — console SSH interactive complète dans le navigateur via xterm.js
- **Binaire unique** — backend Go avec frontend React embarqué ; un seul fichier à déployer, rien d'autre à installer
- **Connexions SSH persistantes** — reconnexion automatique avec backoff exponentiel et keepalive
- **Stockage chiffré des identifiants** — mots de passe serveur chiffrés au repos avec AES-256-GCM
- **Gestion des tâches cron** — consulter, ajouter, modifier, activer/désactiver, supprimer et exécuter immédiatement des tâches cron depuis la vue détaillée du serveur
- **Interface multilingue** — anglais, chinois, français et russe

## Démarrage rapide

Téléchargez le dernier binaire depuis les [Releases](https://github.com/Unmovable8911/Belochka/releases), puis exécutez-le :

```bash
# Linux (amd64)
chmod +x belochka-linux-amd64
./belochka-linux-amd64

# Windows (amd64)
belochka-windows-amd64.exe
```

Ouvrez `http://localhost:53136` dans votre navigateur. Ajoutez des serveurs via l'interface.

## Compilation depuis les sources

Nécessite Go 1.25+ et Node.js 18+.

```bash
git clone https://github.com/Unmovable8911/Belochka.git
cd Belochka
make build
./bin/belochka
```

Compilation croisée des binaires de release pour toutes les plateformes :

```bash
make release
# Produit :
#   bin/belochka-linux-amd64
#   bin/belochka-linux-arm64
#   bin/belochka-windows-amd64.exe
```

## Configuration

Belochka fonctionne directement sans configuration. Vous pouvez optionnellement créer un fichier `belochka.yaml` dans le répertoire de travail ou passer `--config chemin/vers/config.yaml` :

```yaml
port: 53136        # Port d'écoute HTTP (défaut : 53136)
data_dir: ./data   # Emplacement de la base de données et de la clé de chiffrement (défaut : ./data)
encryption_key: "" # Clé AES-256 ; laisser vide pour génération automatique
```

### Variables d'environnement

| Variable | Description |
|---|---|
| `BELOCHKA_ENCRYPTION_KEY` | Remplace la valeur `encryption_key` du fichier de configuration |

### Clé de chiffrement

Au premier lancement sans clé configurée, Belochka en génère une automatiquement dans `{data_dir}/encryption.key` et affiche un avertissement dans les logs. En production, définissez la clé explicitement via le fichier de configuration ou la variable d'environnement.
