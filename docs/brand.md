# Couleurs Exodialabs

La palette provient du site officiel, dépôt `ExodiaLabs/exodialabs-website`,
branche par défaut `develop`, commit
`c65f6dcd91bf7b94634f9ca289b4e8af1e37335d`, consulté le 13 octobre 2026.

Le fichier `docs/design-system.md` décrit le cadre Figma « 00 — Design System »
et indique que `assets/styles/settings/tokens.css` reprend les variables
primitives et sémantiques. Nous avons consulté ces fichiers via un accès GitHub
authentifié en lecture seule après une réponse HTTP 403 du site public. Ces
liens peuvent nécessiter l'accès au dépôt :

- [Système de design](https://github.com/ExodiaLabs/exodialabs-website/blob/c65f6dcd91bf7b94634f9ca289b4e8af1e37335d/docs/design-system.md)
- [Variables CSS](https://github.com/ExodiaLabs/exodialabs-website/blob/c65f6dcd91bf7b94634f9ca289b4e8af1e37335d/assets/styles/settings/tokens.css)

| Primitive officielle | Valeur exacte | Usage terminal |
| --- | --- | --- |
| Violet | `#7960a6` | Accent du thème clair |
| Lavender | `#bba9dc` | Accent du thème sombre |
| White | `#f5f2f9` | Texte sombre, panneaux clairs |
| Prune | `#15111e` | Bandeau sombre, texte clair |
| Void / light void | `#0b0910`, `#ece5f5` | Fond de l'espace de travail sombre / clair |
| Grey | `#8c8697`, `#b4aebe`, `#5e5868` | Texte secondaire et texte de soutien, variantes sombres et claires |
| Anthracite | `#1d1927`, `#27222f`, `#38323f` | Panneaux, cartes et texte secondaire clair |
| Shadow facet | `#543680` | Ligne sélectionnée sombre |
| Mist | `#decdf2` | Ligne sélectionnée claire |
| Glow / deep glow | `#b274f2`, `#7041d1` | Accent complémentaire sombre / clair |

Exodock utilise ces valeurs du site pour séparer le fond, les panneaux, les
cartes et les lignes sélectionnées. Dans le thème sombre, les couleurs d'état
utilisent les rôles ANSI du terminal et conservent un libellé textuel. Le thème
clair affiche les états dans le texte foncé principal en gras pour préserver la
lisibilité. Ces rôles n'ajoutent pas de primitive HEX à la marque. Les terminaux
à palette réduite approchent les valeurs.
`--ascii` et `NO_COLOR` conservent les états et les repères de navigation.
Le thème automatique utilise la réponse du terminal sur son fond ; sans réponse,
Exodock garde le thème sombre. Le terminal contrôle les polices.

[Le guide de l'interface](ui-ux.md) décrit les écrans, le focus clavier et les
préférences d'affichage.
