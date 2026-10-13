# Exodock : proposition de conception

Date : **13 octobre 2026**. Statut : **cadrage de référence du MVP**.

Le MVP a été implémenté après délégation autonome de l'éditeur. Ce document
conserve les intentions et la feuille de route initiales. Les fonctions et
limites livrées figurent dans [le périmètre courant](mvp.md), les tokens de marque
vérifiés dans [la provenance de palette](brand.md), et les ajustements de collecte
dans [l'ADR 0002](adr/0002-collection.md). Les budgets restent à distinguer des mesures.

## 1. Positionnement

Exodialabs propose Exodock comme panneau d'administration Docker au clavier.
L'utilisateur voit l'état de ses ressources, comprend les actions possibles et
obtient un résultat explicite après chaque opération.

Identité : **Exodock**, par **Exodialabs**. Signature :
**Exodock — Docker management, simplified.** Repository cible :
`exodialabs/exodock`. Le nom Exodock domine l'en-tête ; la mention Exodialabs
figure dans l'aide et les informations du produit. La navigation et les états
forment l'identité propre du produit au sein de l'écosystème Exodia.

Une même interface sert les deux publics : menus d'actions avec des verbes pour
les débutants, raccourcis affichés pour les experts. Éviter deux modes qui
feraient diverger les parcours.

## 2. Analyse des solutions existantes

Cette comparaison repose sur les documentations et dépôts publics consultés.
Elle ne constitue pas un benchmark ni un test d'utilisabilité. Les opportunités
ci-dessous représentent notre jugement de conception ; elles ne prouvent pas
l'absence de ces qualités chez les concurrents.

| Solution | Forces documentées | Arbitrage pour Exodock |
| --- | --- | --- |
| Lazydocker | Docker et Compose, logs, graphiques, actions rapides, commandes personnalisables, nettoyage | Reprendre l'accès court aux détails. Faire des libellés d'action, de la cible et des conséquences un élément central du parcours. |
| Dockly | Vue terminal des conteneurs, services et images ; filtres et paramètres de connexion | Retenir la lecture synthétique. Sa distribution Node/npm ajoute une dépendance d'exécution face à notre objectif de binaire autonome. |
| K9s | Navigation par ressources, filtres, aide clavier, contextes, mode lecture seule | Retenir le contexte visible et la recherche. Adapter la densité à Docker et à des débutants. |
| Lazygit | Navigation entre panneaux, détails liés à la sélection, actions contextuelles | Retenir la continuité liste/détail et le retour au même emplacement. |
| Oxker | TUI Rust consacrée aux conteneurs Docker | Inclure ce concurrent dans les essais futurs de fluidité et de consommation mémoire. |

Sources : [Lazydocker](https://github.com/jesseduffield/lazydocker),
[Dockly](https://github.com/lirantal/dockly),
[dépendances de Dockly](https://github.com/lirantal/dockly/blob/main/package.json),
[K9s](https://github.com/derailed/k9s),
[Lazygit](https://github.com/jesseduffield/lazygit),
[Oxker](https://github.com/mrjackwills/oxker).

La différenciation proposée repose sur des comportements mesurables : retrouver
une ressource sans changer d'outil, expliquer une action avant sa confirmation,
préserver la navigation pendant les flux et montrer les échecs partiels.

## 3. Stack recommandée

| Critère | Go + Bubble Tea | Rust + Ratatui |
| --- | --- | --- |
| Intégration Docker | SDK Go officiel | Bollard, client communautaire asynchrone |
| Interface | Architecture état/messages/rendu intégrée ; Bubbles et Lip Gloss | Widgets et layouts riches ; boucle événementielle à organiser avec Crossterm/Tokio |
| Mémoire et latence | GC et runtime à mesurer ; contrôle via buffers et concurrence bornés | Pas de GC ; contrôle fin des allocations, mais les buffers restent à borner |
| Distribution | Binaire compilé, compilation croisée simple lorsque les dépendances évitent CGo | Binaire compilé ; chaînes et dépendances de compilation à valider par cible |
| Maintenance | Outillage homogène, frontières faciles à simuler | Typage fort et ownership ; apprentissage et assemblage plus exigeants sans équipe Rust |

**Recommandation : Go + Bubble Tea v2 + Lip Gloss + Bubbles compatibles.**
La complexité principale vient des flux Docker, des opérations et des états UI.
Le SDK officiel et la boucle de messages réduisent le travail nécessaire pour
ce périmètre. Le nombre d'étoiles ne sert pas de critère de sélection.

Rust reste une alternative solide si l'équipe le maîtrise ou si les mesures
imposent une empreinte que Go ne peut atteindre. Nous ne prétendons pas qu'un
binaire Rust sera forcément plus petit ou qu'une TUI Go sera forcément plus lente.

Autres bibliothèques Go examinées : [tview](https://github.com/rivo/tview),
adapté aux formulaires et widgets ; [termui](https://github.com/gizak/termui),
orienté tableaux de bord. Bubble Tea correspond à notre modèle de transitions
testables et de composants composés.

Références : [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss),
[Bubbles](https://github.com/charmbracelet/bubbles),
[Ratatui](https://ratatui.rs/), [Bollard](https://github.com/fussybeaver/bollard).
L'ADR [0001](adr/0001-stack.md) a été accepté pour le MVP.

## 4. Architecture

Un binaire, un processus, aucune base de données ni serveur Exodock.
Conserver en mémoire les instantanés et les historiques courts. Un fichier
de configuration contient les préférences, sans identifiants de registre.

```text
Clavier / dimensions du terminal
               |
               v
      Boucle UI : état, messages, rendu
               |
               v
      Cas d'usage : requêtes et actions
          /                    \
         v                      v
  Adaptateur Engine       Adaptateur Compose
  SDK Go officiel         CLI officiel, sans shell
         |                      |
         +------> Docker <------+

Résultats, événements et erreurs -> messages -> boucle UI
```

La UI détient son état ; les tâches de collecte lui envoient des résultats.
Le rendu et les transitions n'effectuent aucun appel Docker bloquant. Les cas
d'usage utilisent quelques interfaces ciblées : inventaire, cycle de vie,
flux de logs/statistiques et exécution Compose. Éviter une interface générique
CRUD qui masquerait les différences entre volumes, images et conteneurs.

Chaque requête transporte un identifiant de connexion et de génération.
Une réponse arrivée après un changement de sélection ou de contexte ne doit
pas remplacer les détails courants. Identifier les lignes par l'ID Docker,
jamais par leur position ni uniquement par leur nom.

### Collecte et opérations

- Charger les inventaires en asynchrone, puis écouter les événements Engine.
  Regrouper les rafales et refaire un inventaire périodique pour récupérer les
  événements manqués. Valeur initiale proposée : 5 secondes, configurable.
- Limiter les statistiques aux ressources visibles ou sélectionnées ; étendre
  la collecte pour la vue Ressources avec une concurrence plafonnée. Distinguer
  « total de la sélection » et « ensemble des conteneurs ».
- Utiliser un buffer circulaire pour les logs, plafonné à la fois en lignes
  et en octets : proposition initiale 10 000 lignes / 8 Mio pour le flux actif.
  Borner aussi les lignes individuelles et le travail de décodage.
- Remplacer les échantillons statistiques obsolètes dans la file ; conserver
  les résultats d'opérations dans un canal séparé fiable. Signaler les lignes
  perdues si un producteur dépasse la capacité de traitement.
- Borner les historiques de métriques, les files, les tâches et les connexions.
  Arrêter les abonnements quand leur vue n'en a plus besoin.
- Appliquer des délais aux requêtes courtes. Pour un arrêt, intégrer le délai
  de grâce Docker ; pour un téléchargement, distinguer inactivité et durée totale.
- Reconnecter les lectures avec temporisation et refaire l'inventaire.
  Après un timeout d'écriture, vérifier l'état et afficher « résultat à vérifier »
  si nécessaire. Ne pas rejouer une suppression à l'aveugle.
- Quitter via une annulation commune, fermer lecteurs et processus, restaurer
  écran alternatif et curseur. Une interruption peut laisser une opération
  déjà acceptée par le daemon en cours ; vérifier son résultat au retour.

### Connexion Docker

Employer le SDK officiel actuellement documenté sous
`github.com/moby/moby/client`, avec négociation de version API. Ne pas coder
uniquement `/var/run/docker.sock` : Desktop et rootless peuvent utiliser un
socket utilisateur. [Documentation SDK](https://docs.docker.com/reference/api/engine/sdk/).

Un résolveur de connexion explicite applique les priorités proposées : option
Exodock `--context` ou `--host` (mutuellement exclusives), puis `DOCKER_CONTEXT`,
puis `DOCKER_HOST`, puis contexte Docker courant. Le CLI Docker documente la
priorité de `DOCKER_CONTEXT` sur `DOCKER_HOST`.
[Référence CLI](https://docs.docker.com/reference/cli/docker/).

Utiliser les commandes JSON du CLI officiel pour lire les contextes, derrière
une interface testable. Le SDK seul n'interprète pas automatiquement toutes
leurs métadonnées. Sans CLI, accepter une adresse explicite ou un socket local
standard en annonçant la cible avant toute mutation. Une résolution invalide
produit une erreur ; ne pas basculer vers un autre daemon.

Le MVP prend en charge les connexions locales Unix pour Linux, WSL et macOS.
Reconnaître une adresse distante ou un transport non pris en charge et
expliquer cette limite. TLS/SSH et changement interactif de contexte arrivent
après le MVP. La UI affiche en permanence cible et état de connexion.
[Contextes Docker](https://docs.docker.com/engine/manage-resources/contexts/).

### Docker Compose

Lire les labels officiels `com.docker.compose.project` et
`com.docker.compose.service` pour regrouper les ressources existantes.
Cette vue ne permet pas de découvrir un projet jamais créé ni de reconstruire
son fichier source. [Labels Compose](https://docs.docker.com/reference/compose-file/services/#labels).

Pour les futures actions, utiliser le plugin Compose officiel installé et
ses capacités détectées. Éviter d'embarquer son moteur dans Exodock. Lier le
projet à des fichiers locaux choisis par l'utilisateur ; traiter les chemins
contenus dans des labels comme des informations non fiables.

Transmettre arguments, contexte et environnement de façon contrôlée, avec
`exec.CommandContext` et sans `sh -c`. Borner et assainir les sorties.
Ne pas changer le contexte global de l'utilisateur. Avant d'agir, vérifier
l'alignement entre la connexion Engine et celle de Compose.

Séparer les intentions : « Démarrer les services existants » correspond à
`compose start`, « Arrêter » à `compose stop`, « Redémarrer » à `compose restart`.
« Créer / appliquer le projet » utilise `compose up -d` dans un lot ultérieur,
avec aperçu des conséquences : cette commande peut recréer des conteneurs.
[start](https://docs.docker.com/reference/cli/docker/compose/start/),
[up](https://docs.docker.com/reference/cli/docker/compose/up/).

## 5. Identité visuelle et contraintes terminal

Le site `exodialabs.dev` refuse l'accès automatisé lors de cette recherche
(HTTP 403). L'accès GitHub authentifié a ensuite permis de vérifier la charte
dans le dépôt officiel du site, sur son commit de référence. Exodock reprend
les HEX exacts de [cette palette](brand.md), dont le violet `#7960a6`, la lavande
`#bba9dc`, le prune `#15111e` et le blanc `#f5f2f9`.

| Rôle de token | Usage prévu |
| --- | --- |
| `brand.primary` | Violet désaturé : section active, focus, action principale |
| `surface.base`, `surface.panel` | Fond sombre anthracite et séparation discrète |
| `text.primary`, `text.muted`, `border` | Texte, métadonnées et structure en gris neutres |
| `status.success`, `status.warning`, `status.error` | États doux, avec texte et symbole ; couleurs à valider |
| Variantes claires | Mêmes rôles et hiérarchie, contrastes adaptés |

La couleur accompagne le sens. Afficher « En cours », « Arrêté », « Erreur »
et l'état de santé séparément. « En cours » ne garantit pas « Sain ».
Réserver le violet à l'identité et au focus, éviter les surfaces saturées.

Exodock utilise la police monospace du terminal ; il ne peut pas la choisir.
Tester la largeur des cellules, la troncature et les caractères combinés.
Éviter emoji et dépendance à Nerd Fonts. Proposer symboles Unicode sobres et
alternatives ASCII : `●` / `*`, `✓` / `OK`, `!`, bordures `+|-`.

Prévoir truecolor, 256 couleurs, 16 couleurs et monochrome. Les modes limités
approchent les tokens officiels sans prétendre reproduire leurs HEX.
Respecter `NO_COLOR`, proposer `--ascii`, `--theme dark|light|auto` et un réglage
qui désactive les animations. Le thème clair complet rejoint le lot de finition.

### Composition adaptative

| Taille disponible | Présentation |
| --- | --- |
| Au moins 120 colonnes et 30 lignes | Navigation, tableau et panneau de détails côte à côte |
| Au moins 80 colonnes et 24 lignes | Navigation compacte, tableau, détails en vue remplaçante |
| Au moins 60 colonnes et 18 lignes | Colonnes prioritaires, vues empilées, menu de sections |
| En dessous | Message de dimensions insuffisantes et sortie accessible |

Mesurer largeur et hauteur. Supprimer d'abord les graphiques et métadonnées
secondaires ; conserver nom, état et actions. La sélection, le filtre et le
défilement survivent aux redimensionnements. Sous une modale, le focus reste
dans la modale. Un terminal non interactif reçoit un message CLI utile.

## 6. Écrans et navigation

Sections : **Vue d'ensemble, Conteneurs, Images, Volumes, Réseaux, Compose,
Ressources, Système**. Logs et inspection restent liés à la ressource choisie.
La recherche globale et l'aide s'ouvrent au-dessus de la vue courante.

### Vue d'ensemble

Afficher connexion, conteneurs actifs/arrêtés/en erreur, ressources recensées,
consommation disponible et espace Docker. Donner priorité aux anomalies et aux
dernières opérations. Chaque indicateur mène à une liste filtrée.

Éviter une somme naïve des tailles d'images qui compterait plusieurs fois les
couches partagées. La vue Système utilise les informations de disque Docker
à la demande, avec date de mesure. Ne pas appeler cela « espace libre de
l'hôte » si l'API ne le fournit pas.
[docker system df](https://docs.docker.com/reference/cli/docker/system/df/).

### Conteneurs et détails

Maquette structurelle pour terminal large ; données fictives, aucune palette
implicite :

```text
+---------------------------------------------------------------------------------------------+
| EXODOCK                     Contexte : desktop-linux                 Daemon : connecté      |
+-------------------+-------------------------------------------+-----------------------------+
| Vue d'ensemble    | CONTENEURS     Filtre : projet=exolan     | api-1                       |
| > Conteneurs      |                                           | En cours / Sain             |
| Images            |   NOM        ÉTAT      CPU    MÉMOIRE     | Image : exolan/api:dev      |
| Volumes           | > api-1      En cours  1.2%   184 Mio     | Ports : 8080 -> 80          |
| Réseaux           |   db-1       En cours  0.4%   256 Mio     | Projet : exolan             |
| Compose           |   worker-1   Arrêté    --     --          |                             |
| Ressources        |                                           | Résumé  Logs  Stats  Inspect |
| Système           |                                           |                             |
|                   |                                           | [a] Actions                 |
+-------------------+-------------------------------------------+-----------------------------+
| / Filtrer  Ctrl+k Rechercher  Entrée Détails  a Actions  ? Aide  q Quitter                    |
+---------------------------------------------------------------------------------------------+
```

Priorité des colonnes : nom, état, santé, CPU, mémoire, ports, projet, ancienneté.
L'état de santé reste distinct et peut être absent. Les onglets du panneau
exposent résumé, logs, statistiques et inspection avec valeurs sensibles masquées.
Une action indisponible reste visible dans le menu avec sa raison.

### Autres sections

| Section | Données et actions |
| --- | --- |
| Images | Tags, ID, taille Docker, usage ; détails ; téléchargement et suppression dans le lot suivant |
| Volumes | Nom, driver, références de conteneurs ; inspection ; taille « inconnue » si indisponible ; suppression ultérieure |
| Réseaux | Nom, driver, portée, connexions ; inspection ; création bridge et suppression ultérieures |
| Compose | Projets détectés, services, réplicas et santé ; détails de conteneur ; actions de projet ultérieures |
| Ressources | CPU, mémoire, trafic et historiques courts ; périmètre de collecte affiché |
| Système | Version daemon/API, plateforme du daemon, capacité CPU/mémoire disponible, inventaire et usage disque Docker |

Les métriques décrivent le daemon Docker, qui peut tourner dans la VM Desktop.
Ne pas les présenter comme des métriques physiques du Mac ou de Windows.
CPU peut dépasser 100 % avec plusieurs cœurs ; documenter sa normalisation.
Présenter mémoire selon la plateforme, compteurs réseau et débits calculés par
différence. Afficher « indisponible » pour les champs absents.
[Statistiques Docker](https://docs.docker.com/reference/cli/docker/container/stats/).

### Logs

Suivi en temps réel, pause de défilement, retour au suivi, recherche de texte,
filtrage stdout/stderr et plage temporelle. La recherche porte sur le buffer
chargé, avec cette limite visible. Un filtre de sévérité viendra plus tard pour
les formats reconnus ; Docker ne fournit pas un niveau universel de log.

Distinguer les flux multiplexés des conteneurs sans TTY et le flux brut avec
TTY. Signaler un driver qui ne permet pas la lecture. Assainir les caractères
de contrôle avant rendu ; conserver les retours de ligne utiles.

### Raccourcis proposés

| Touche | Intention |
| --- | --- |
| Flèches ou `j` / `k` | Déplacer la sélection |
| `Tab` / `Shift+Tab` | Changer de panneau |
| `Entrée` | Ouvrir les détails ou valider le contrôle actif |
| `Esc` | Fermer, annuler la saisie ou revenir |
| `/` | Filtrer la liste ; chercher dans la vue Logs |
| `Ctrl+k` | Rechercher une ressource dans le contexte actif |
| `:` | Palette d'actions et de navigation |
| `a` | Menu des actions de la ressource |
| `l` / `i` | Logs / inspection dans une liste compatible |
| `r` | Rafraîchir ; le redémarrage passe par le menu d'actions |
| `Espace` | Sélection groupée dans les listes qui la supportent |
| `?` | Aide contextuelle |
| `q` / `Ctrl+c` | Quitter et libérer le terminal |

Les champs texte capturent leurs caractères sans lancer d'action. Le menu
et l'aide montrent les raccourcis actifs. Ajouter des raccourcis d'expert
reconfigurables après stabilisation ; une touche destructive ouvre toujours
une confirmation. Une modale laisse `Esc` annuler et `Ctrl+c` quitter.

### Parcours de référence

1. **Découvrir Docker** : ouvrir Exodock, lire la cible et l'état du daemon,
   ouvrir Conteneurs, choisir une ressource, ouvrir Actions, choisir Démarrer.
   Voir « en cours », puis le résultat réel. En cas d'échec, conserver la
   sélection et proposer logs ou diagnostic adapté.
2. **Diagnostiquer un service** : rechercher `api`, ouvrir les logs, filtrer
   une erreur, consulter les stats, ouvrir Redémarrer, confirmer l'interruption,
   puis retrouver le suivi des logs. Signaler les ruptures de flux.
3. **Supprimer un conteneur arrêté** : ouvrir Supprimer, lire le nom, l'ID et
   la cible, vérifier l'impact sur sa couche inscriptible, confirmer. Exodock
   garde les volumes ; Docker peut refuser si la situation a changé.
4. **Daemon inaccessible** : afficher la cible et une cause utile : socket
   absent, permission refusée, daemon indisponible ou API incompatible. Proposer
   Réessayer, Aide ou Quitter. Aucune relance par sudo.

## 7. Actions et sécurité

| Opération | Protection prévue |
| --- | --- |
| Démarrer | Action explicite ; suivi du résultat |
| Arrêter / redémarrer | Confirmation de l'interruption et délai de grâce visible |
| Supprimer un conteneur arrêté | Confirmation nominative ; sans force ni suppression des volumes |
| Supprimer une image | Montrer tags/références et refus Docker éventuel ; pas de force implicite |
| Supprimer un volume | Montrer les références connues ; saisir son nom ; préciser la perte de données |
| Supprimer un réseau | Montrer les connexions ; exclure les réseaux intégrés ; confirmation |
| Action groupée | Figer les IDs et la cible, résumer le nombre, lister les exclus, montrer un résultat par ressource |

La confirmation utilise « Annuler » par défaut. Rafraîchir les préconditions
avant exécution ; Docker décide en dernier ressort et peut retourner un conflit.
Une liste de ressources sans référence connue ne garantit pas qu'elles soient
sans valeur : un volume inutilisé peut contenir des données importantes.
Ne pas annoncer de restauration possible après une suppression.

Un mode `--readonly` refuse les mutations dans les cas d'usage et les adaptateurs,
en plus de désactiver les contrôles. Il constitue une protection Exodock,
pas une restriction de permissions imposée par le daemon.

Docker accorde des capacités puissantes aux utilisateurs autorisés sur son
socket. Exodock fonctionne avec leurs permissions existantes, sans modifier
groupes, permissions ou privilèges. Les futurs transports distants utilisent
TLS vérifié ou SSH ; pas de connexion distante HTTP en clair.
[Sécurité du daemon](https://docs.docker.com/engine/security/).

Les diagnostics Exodock enregistrent uniquement une liste de champs autorisés :
type d'opération, ID technique, durée, classe d'erreur et résultat. Exclure
payloads d'inspection, environnement, sorties de conteneur, auth de registre,
certificats et arguments potentiellement sensibles. Aucun upload automatique.

Masquer les valeurs d'environnement et champs sensibles de l'inspection par
défaut ; limiter les champs affichés et traiter aussi labels et commandes comme
potentiellement sensibles. Une révélation locale explicite ne persiste rien.
Les logs applicatifs peuvent contenir des secrets impossibles à identifier avec
certitude : leur lecteur reste en mémoire et ne les recopie pas dans les logs
internes. Un masquage heuristique ne vaut pas garantie d'anonymisation.

Traiter noms, labels, logs et messages du daemon comme des entrées non fiables :
neutraliser ANSI/OSC, contrôles du curseur, liens et commandes de presse-papiers.
Vérifier le même traitement sur les sorties Compose et les erreurs.

Erreurs produit : cause courte, ressource concernée, action suivante, détail
technique assaini accessible. Exemples : « Docker est inaccessible sur cette
cible. Vérifiez que le daemon est démarré, puis réessayez. » ; « Ce volume est
encore référencé par un conteneur. Consultez ses références. »

## 8. MVP et suites

Le MVP **v0.1** permet de diagnostiquer Docker et de gérer le cycle de vie
d'un conteneur local. Toutes les catégories sont consultables ; les mutations
plus complexes rejoignent des lots distincts.

| Domaine | v0.1 MVP | v0.2 Administration | v0.3 Finition et connexions |
| --- | --- | --- | --- |
| Vue d'ensemble | État, inventaire, anomalies | Résumé des actions groupées | Préférences de disposition |
| Conteneurs | Liste, filtres, détails, start/stop/restart, suppression à l'arrêt | Actions groupées | Raccourcis configurables |
| Images | Liste et inspection | Pull public/privé avec helpers Docker, progression, suppression | Filtres avancés |
| Volumes | Liste et inspection | Suppression avec saisie du nom | Analyse d'usage plus détaillée |
| Réseaux | Liste et inspection | Création bridge et suppression | Options réseau avancées selon besoin |
| Compose | Projets/services détectés, détails | Start/stop/restart avec fichiers locaux liés | Application explicite via up -d |
| Logs | Un conteneur, suivi/pause, texte, flux, période | Plusieurs services, regex bornées | Parsers de formats reconnus |
| Ressources | Stats de la sélection, historique court | Vue comparative avec collecte limitée | Budgets et présentation ajustables |
| Système | Daemon/API, capacités et usage disque à la demande | Améliorations selon retours | Diagnostic de connexions distantes |
| Recherche/aide | Globale sur inventaires en mémoire, noms/IDs/tags/projets ; aide | Labels et filtres combinés | Personnalisation |
| Compatibilité | Linux, WSL, macOS ; Unix local ; ASCII et monochrome | Stabilisation et RC Windows | Windows validé, SSH/TLS, thème clair complet |

Le téléchargement privé réutilisera la configuration et les credential helpers
Docker via un adaptateur ; aucun mot de passe enregistré par Exodock. Sa prise
en charge demandera des tests dédiés avant publication.

Reporter hors de ces versions : shell interactif/exec, éditeur YAML, build
d'images, Swarm/Kubernetes, plugins, gestion simultanée de plusieurs daemons,
prune global et suppression forcée. Ces fonctionnalités ajoutent des risques
ou de la complexité sans être nécessaires aux parcours initiaux.

### Critères de sortie du MVP

- Effectuer les quatre parcours de référence au clavier et sous petit terminal.
- Montrer toute suppression dans une confirmation qui fige ID et cible.
- Refuser les mutations en lecture seule, même en appelant directement un cas d'usage.
- Naviguer pendant un daemon lent, un flux abondant ou une reconnexion.
- Rester dans les plafonds de buffers et ne pas afficher de données périmées
  comme actuelles : date de collecte et état « connexion perdue » visibles.
- Restaurer le terminal après sortie normale, signal et erreur contrôlée.
- Ne pas écrire les marqueurs secrets de nos fixtures dans les diagnostics.
- Vérifier Linux et Docker Desktop macOS, plus WSL via une recette manuelle.
  La compilation seule ne constitue pas une validation Docker multiplateforme.

### Budgets initiaux à mesurer

Objectifs, **aucune mesure obtenue à ce stade** : réponse clavier p95 sous
100 ms, mémoire au repos sous 64 Mio et binaire distribué sous 25 Mio par cible.
Référence de test : machine 4 cœurs / 8 Gio, 100 conteneurs synthétiques,
une vue active, un flux plafonné à 1 000 lignes/s de 200 octets et historique
court. Publier taille brute/compressée, RSS, allocations et utilisation CPU.

Mesurer séparément collecte et rendu, puis répéter avec 1 000 ressources.
Le stress doit préserver les commandes clavier, borner la mémoire et signaler
la perte de logs. Le temps d'une opération Docker n'entre pas dans le budget
de réaction clavier. Ajuster les seuils sur les mesures du prototype.

## 9. Qualité, dépôt et distribution

Arborescence cible, à créer lors du développement :

```text
cmd/exodock/                 point d'entrée et options CLI
internal/app/                cas d'usage, coordination, règles d'action
internal/docker/             client Engine et conversion des réponses
internal/compose/            adaptateur du CLI Compose
internal/connection/         résolution de cible et capacités
internal/tui/                modèle, écrans, composants, clavier, thème
internal/config/             préférences versionnées
internal/diagnostics/        erreurs et journal assaini
test/integration/            tests avec daemon éphémère
testdata/                    inventaires, flux et données fictives
docs/adr/                    décisions proposées puis acceptées
.github/workflows/           CI et releases
.github/ISSUE_TEMPLATE/      bugs et propositions
README.md
CONTRIBUTING.md
SECURITY.md
CODE_OF_CONDUCT.md
LICENSE
go.mod et go.sum
```

Créer les modules quand leur responsabilité existe, pas des répertoires vides
pour anticiper tous les domaines. Utiliser les types Docker à l'intérieur de
l'adaptateur ; exposer aux écrans des modèles courts adaptés à leurs besoins.
Éviter une couche repository générique, un bus distribué et l'injection via framework.

### Tests et CI GitHub Actions

1. **Unitaires** : filtres, calculs de métriques, priorités de connexion,
   transitions UI, focus des modales, confirmations, annulation et réponses périmées.
2. **Contrats** : serveur HTTP simulé pour erreurs, négociation API, flux coupés,
   multiplexage ; faux runner pour arguments Compose et propagation de cible.
3. **Intégration Linux** : daemon de test isolé, ressources avec préfixe/labels
   de test, cycle de vie et cleanup limité aux IDs créés. Aucun prune global.
4. **Terminal** : snapshots de cellules à plusieurs tailles, tests PTY pour
   touches, Unicode, signaux et restauration. Fixtures d'injection ANSI/OSC.
5. **Recette réelle** : macOS Desktop, WSL, tmux/SSH côté terminal, mode ASCII,
   monochrome ; Windows Terminal et named pipes avant annoncer Windows stable.

À chaque PR : formatage Go, `go vet`, analyse statique, tests unitaires/contrats,
tests avec race detector sur une cible compatible, intégration Docker Linux et
build des cibles annoncées. Ajouter analyse des vulnérabilités et dépendances,
contrôle Markdown/liens locaux et comparaison des tailles du binaire.

Épingler les actions à leur SHA, limiter les permissions CI, garder les tests
de PR sans secrets et réserver la publication au workflow de tag protégé.
Ne pas exécuter du code de fork via un workflow privilégié.

Releases initiales : archives Linux/macOS amd64/arm64 avec checksums et
provenance/SBOM ; Windows expérimental seulement après essais named pipes.
Prévoir Homebrew/Scoop quand les releases sont stables. Pas de commande
d'installation fictive avant publication.

Le guide de développement précisera version Go épinglée, prérequis Docker,
commande de lancement, tests sans daemon et tests d'intégration isolés.
La documentation publique initiale peut être en anglais pour l'open source ;
le cadrage produit reste en français. Les libellés présentés ici décrivent les
intentions ; une première UI anglaise conservera des verbes simples. Une
localisation française complète rejoint une évolution dédiée.

**Licence proposée : MIT**, pour une adoption simple, y compris commerciale.
Exodialabs conserve la marque ; l'usage du nom/logo mérite une règle distincte
dans CONTRIBUTING. Apache-2.0 serait une alternative si l'éditeur souhaite une
concession explicite de brevets. Vérifier les notices des dépendances lors de
la release. [Licence MIT](https://choosealicense.com/licenses/mit/).

## 10. Plan progressif

Estimation indicative : **6 à 8 semaines pour une personne expérimentée à temps
plein**, avec retours UX disponibles, pour v0.1. Ce budget inclut recette et
documentation ; il ne comprend pas v0.2/v0.3.

| Lot | Travail | Critère de passage |
| --- | --- | --- |
| 0 · Cadrage | Décisions, tokens de marque, maquettes et scénarios | Stack/MVP stabilisés ; source de palette identifiée ou branding laissé provisoire |
| 1 · Prototype, 1 semaine | Boucle UI, layouts, clavier, données fictives, mesures | Focus et redimensionnement cohérents ; budgets initiaux évalués |
| 2 · Lecture Docker, 1 à 2 semaines | Résolution locale, inventaires, dashboard, détails, erreurs | Bon daemon, absence de blocage, reconnexion testée |
| 3 · Actions conteneurs, 1 semaine | Cycle de vie, confirmations, mode lecture seule | Refus Docker, timeout ambigu et état modifié couverts |
| 4 · Flux et diagnostic, 1 à 2 semaines | Logs, stats, système, regroupement Compose, recherche | Buffers bornés, données datées, rafales supportées |
| 5 · Release v0.1, 1 à 2 semaines | Tests de terminaux/OS, CI, docs, licence, archives | Critères du MVP vérifiés ; limites de compatibilité publiées |
| 6 · v0.2 | Pull, suppressions, réseaux bridge, Compose, actions groupées | Chaque opération apporte son aperçu et sa recette d'échec |
| 7 · v0.3 | Thème clair, Windows, connexions distantes et raccourcis | Transport et plateforme validés avant annonce de support |

Avant de choisir les versions de dépendances, confronter le prototype Go aux
budgets et à deux parcours avec des débutants et des experts. Mesurer le temps
pour trouver une ressource, ouvrir ses logs et terminer une action sans aide.
Utiliser ces essais pour régler densité, libellés et raccourcis.

Le dépôt contient maintenant le MVP et son mode démo sur données fictives.
Les phases suivantes portent sur la recette des plateformes supplémentaires
et les fonctions v0.2 ; consulter [le périmètre courant](mvp.md).
