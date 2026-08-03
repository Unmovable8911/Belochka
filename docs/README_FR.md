<p align="center">
  <img src="../logo.png" width="500">
</p>
<p align="center">
<a href="./README_DE.md">Deutsch</a> / 
<a href="../README.md">English</a> / 
<a href="./README_ES.md">Español</a> / 
Français / 
<a href="./README_IT.md">Italiano</a> / 
<a href="./README_PT.md">Português</a> / 
<a href="./README_RU.md">Русский</a> / 
<a href="./README_CN.md">中文</a> / 
<a href="./README_TW.md">繁體中文</a>
</p>
<hr>

Belochka (белочка, « écureuil ») est un outil de surveillance serveur mono-binaire conçu pour de petites flottes de serveurs Linux. Il maintient des connexions SSH persistantes vers 5 à 20 machines distantes et diffuse en temps réel les métriques CPU, mémoire, disque, réseau et processus vers un tableau de bord navigateur via WebSocket. Il fournit également un terminal interactif web pour un accès SSH direct — aucun client SSH séparé requis.

## Fonctionnalités

- **Groupes de serveurs** — organisez les serveurs en groupes plats de niveau racine dans la barre latérale ; créez, renommez et supprimez des groupes, déplacez les serveurs dans ou hors d'un groupe par glisser-déposer ou via le menu « Déplacer vers… » ; cliquez sur un groupe pour filtrer le tableau de bord sur ses membres, avec la navigation par fil d'Ariane
- **Tableau de bord en temps réel** — cartes serveur avec métriques CPU, mémoire, disque et réseau en direct, colorées selon l'utilisation ; clic droit sur une carte pour des actions rapides Modifier / Supprimer / Console
- **Vue détaillée du serveur** — jauges CPU par cœur, graphiques en anneau mémoire/swap, détail des partitions disque, débit par interface réseau
- **Gestion des processus** — onglet Processus avec un tableau plat triable (colonnes redimensionnables), recherche multi-mots-clés, actualisation automatique, terminaison avec sélection du signal SIGTERM/SIGKILL (sshd/init/systemd protégés)
- **Terminal web** — console SSH interactive complète dans le navigateur via xterm.js
- **Icône dans la barre système** — sur les machines de bureau (Windows, macOS, Linux avec GNOME/KDE/XFCE), affiche une icône avec les entrées **Ouvrir le tableau de bord** et **Quitter** ; bascule automatiquement en mode CLI sur les serveurs sans interface graphique
- **Authentification** — protection par mot de passe et cookie de session ; premier accès guidé par un assistant en deux étapes (choisir la langue → définir un mot de passe avec indicateur de force et confirmation) ; limitation après 10 échecs de connexion (verrouillage de 30 minutes)
- **Binaire unique** — backend Go avec frontend React embarqué ; un seul fichier à déployer, rien d'autre à installer
- **Connexions SSH persistantes** — reconnexion automatique avec backoff exponentiel et keepalive ; testez la connexion avant d'enregistrer un serveur, et l'empreinte de la clé d'hôte est affichée pour une vérification trust-on-first-use lors de l'ajout d'une machine
- **Téléversement de clés via le navigateur** — téléversez des fichiers de clé privée SSH directement via l'interface ; les clés sont validées, stockées avec des noms UUID, et les fichiers orphelins sont automatiquement nettoyés
- **Stockage chiffré des identifiants** — mots de passe serveur chiffrés au repos avec AES-256-GCM
- **Gestion des tâches cron** — consulter, ajouter, modifier, activer/désactiver, supprimer et exécuter immédiatement des tâches cron depuis la vue détaillée du serveur
- **Commande par lots** — rédigez un script multi-lignes et envoyez-le à un ensemble de serveurs depuis la barre latérale ; la sortie terminal de chaque serveur s'affiche en direct dans la boîte de dialogue, vous pouvez interagir avec les invites et annuler l'exécution à tout moment
- **Fichier journal persistant** — toutes les sorties écrites dans `belochka.log` à côté du binaire (ou dans le répertoire de travail courant avec `go run`) avec nettoyage automatique basé sur la rétention (défaut : 3 jours)
- **Interface multilingue** — anglais, chinois simplifié, français, russe, allemand, espagnol, portugais, chinois traditionnel et italien ; sélectionnable lors de la configuration initiale et modifiable depuis la boîte de dialogue Paramètres ; langue détectée par le navigateur pré-sélectionnée
- **Paramètres intégrés** — configurez le port, le répertoire de données, la langue et la rétention des journaux directement depuis l'icône engrenage du tableau de bord, sans modifier de fichier de configuration

## Démarrage rapide

Téléchargez le dernier binaire depuis les [Releases](https://github.com/Unmovable8911/Belochka/releases), puis exécutez-le :

```bash
# Linux (amd64)
chmod +x belochka-linux-amd64
./belochka-linux-amd64

# Windows (64 bits)
belochka-windows-x86-64.exe
```

Ouvrez `http://localhost:53136` dans votre navigateur. Lors de la première visite, vous serez invité à choisir votre langue et à définir un mot de passe — celui-ci protège le tableau de bord et tous les endpoints de l'API. Une fois la configuration terminée, vous êtes automatiquement connecté. Ajoutez des serveurs via l'interface.

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
#   bin/belochka-windows-x86-64.exe
#   bin/belochka-windows-x86.exe
```

## Configuration

Belochka fonctionne directement sans configuration. Tous les paramètres sont accessibles via la **boîte de dialogue Paramètres** (icône engrenage dans l'en-tête du tableau de bord). Un fichier `config.json` avec les valeurs par défaut est automatiquement créé à côté du binaire au premier lancement. Vous pouvez également passer `--config chemin/vers/config.json` pour un emplacement personnalisé :

```json
{
  "port": 53136,
  "data_dir": "./data",
  "language": "",
  "log_path": "",
  "log_retention_days": 3
}
```

| Champ | Défaut | Description |
|---|---|---|
| `port` | `53136` | Port d'écoute HTTP |
| `data_dir` | `./data` | Emplacement de la base de données, de la clé de chiffrement et des fichiers de clé SSH téléversés (`data/keys/`) |
| `language` | `""` | Langue de l'interface (`en`, `zh`, `fr`, `ru`, `de`, `es`, `pt`, `zh-TW`, `it`) ; détectée automatiquement à la première visite si vide |
| `log_path` | `""` | Chemin du fichier journal ; utilise `belochka.log` à côté du binaire si vide |
| `log_retention_days` | `3` | Nombre de jours de conservation des entrées de journal |

Les modifications de `port` et `data_dir` nécessitent un redémarrage ; `language` et `log_retention_days` s'appliquent immédiatement via la boîte de dialogue Paramètres.

### Arguments

| Argument | Description |
|---|---|
| `--config <chemin>` | Chemin vers le fichier de configuration JSON |
| `--no-tray` | Désactive l'icône dans la barre système ; lance en tant que processus CLI |
| `--version` | Affiche la version et quitte |

### Variables d'environnement

| Variable | Description |
|---|---|
| `BELOCHKA_ENCRYPTION_KEY` | Clé AES-256 pour le chiffrement des mots de passe ; générée automatiquement au premier lancement si non définie |

### Clé de chiffrement

Au premier lancement sans clé définie, Belochka en génère une automatiquement dans `{data_dir}/encryption.key` et affiche un avertissement dans les logs. En production, définissez la clé explicitement via la variable d'environnement.
