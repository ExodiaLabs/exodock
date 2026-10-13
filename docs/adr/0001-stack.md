# 0001 : Go, Bubble Tea et SDK Docker officiel

- **Statut : accepté**
- **Date : 2026-10-13**
- **Décision : délégation autonome d'Exodialabs pour le MVP**
- **Portée : application locale, premier MVP**

## Contexte

Exodock doit proposer une TUI réactive, un binaire autonome et une intégration
Docker fiable. Le projet commence sans code ni contrainte de stack héritée.
La gestion des flux, les permissions du daemon et le comportement des
opérations priment sur le coût du rendu d'un tableau.

## Décision proposée

Utiliser Go, Bubble Tea v2 et Lip Gloss, avec des composants locaux ciblés.
Le MVP n'a pas besoin d'ajouter Bubbles pour ses champs courts et tableaux.
Épingler Bubble Tea v2.1.0, Lip Gloss v2.0.6 et le SDK Go officiel
`github.com/moby/moby/client` v0.6.2. La CI utilise Go 1.27.2.

Conserver un seul processus, un état UI détenu par la boucle de messages et
des opérations asynchrones annulables. Ajouter des interfaces uniquement aux
frontières Docker, Compose, résolution de connexion et exécution de processus.

Lire les projets Compose via les labels Docker. Pour les actions de projet,
appeler le plugin Compose officiel via un adaptateur dédié, sans shell.
Les actions Compose rejoignent le lot suivant le MVP.

## Conséquences

Le SDK officiel et l'outillage Go réduisent le travail d'intégration. Le modèle
de messages facilite les tests des transitions et des réponses asynchrones.
Le ramasse-miettes et la taille du runtime imposent de mesurer mémoire et
latence. Le projet borne ses flux, ses historiques et ses requêtes concurrentes.

Le CLI Docker reste une dépendance conditionnelle pour résoudre ses contextes
et une dépendance pour les futures actions Compose. Le SDK ne remplace pas
ces fonctions. Aucun benchmark comparatif n'a encore été exécuté.

## Alternatives

- **Rust, Ratatui, Crossterm, Tokio et Bollard** : pertinent pour un budget mémoire
  strict et une équipe Rust ; demande davantage d'assemblage et utilise un
  client Docker communautaire. Garder cette option si le prototype Go manque
  les budgets mesurés malgré une collecte bornée.
- **Go et tview/tcell** : widgets adaptés à une console d'administration ;
  Bubble Tea correspond mieux à la séparation état/messages/rendu retenue.
- **Node.js et Blessed** : adapté à une équipe JavaScript, avec une distribution
  qui ajoute le runtime Node ; moins aligné avec le binaire autonome recherché.

## Références

- [SDK Docker officiel](https://docs.docker.com/reference/api/engine/sdk/)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Ratatui](https://ratatui.rs/)
- [Bollard](https://github.com/fussybeaver/bollard)
- [Conception détaillée](../conception.md)
