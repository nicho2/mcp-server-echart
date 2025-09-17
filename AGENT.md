# Agent mcp-server-echarts
Agent MCP générant des pages ECharts statiques pour les clients LLM de l'organisation nicho2.
Mission : transformer une configuration JSON en URL d'affichage responsive sans dépendre d'un front dédié.
Périmètre : rendu graphique, persistance locale des HTML, mise à disposition HTTP via Go/mcp-go.
Valeur ajoutée : industrialise la génération de visualisations pour les équipes data produit avec traçabilité.
Statut : bêta contrôlée (enrôlement par DevOps uniquement).

## Architecture fonctionnelle de l'agent
```mermaid
flowchart TD
    U[Client MCP / orchestrateur LLM] -->|StreamableHTTP POST /mcp| EP[Routeur HTTP<br/>(`main.go`)]
    EP -->|Validation schéma & guardrails| Policy[Couche Policy<br/>(`GenerateEchartsPage`)]
    Policy --> Tool[Tool Handler<br/>`generate_echarts_page`]
    Tool -->|Rendu HTML| FS[(Stockage statique<br/>`./static/charts`)]
    Tool -->|ToolResult JSON| Resp[Réponse MCP<br/>(URL du HTML)]
    FS -->|HTTP GET /charts/...| U
```

**Composants clés**
- **Routeur HTTP unique** : `http.Server` + `http.ServeMux` exposant `/mcp` (MCP StreamableHTTP) et `/` pour les fichiers statiques ([`main.go`](./main.go)).
- **Couche policy/validation** : vérification stricte des types d'arguments (`inputSchema`, `title`, `width`, `height`).
- **Tool handler** : fonction [`GenerateEchartsPage`](./main.go) sérialisant la configuration, remplissant [`template.html`](./template.html) et persistant le HTML.
- **Stockage statique** : arborescence `STATIC_DIR/charts` (par défaut `static/charts`) servie en lecture seule.
- **Logs JSON** : `logrus` (niveau paramétrable) pour audit et observabilité.

**Flux de données**
1. Le client MCP envoie un appel `generate_echarts_page` via StreamableHTTP `/mcp`.
2. La couche policy valide le format JSON, normalise les dimensions et enrichit la configuration ECharts.
3. Le handler rend la page HTML avec un nom horodaté, l'enregistre puis calcule l'URL publique à retourner.
4. La réponse MCP contient l'URL ; l'utilisateur télécharge ensuite la page via le serveur statique.

## Capacités & limites

**Capacités majeures**
- Génération déterministe de pages HTML ECharts à partir d'un JSON ECharts 5.x.
- Ajout automatique de composants UI (toolbox, tooltip, toggle de configuration) pour améliorer l'expérience.
- Journalisation structurée des appels et erreurs pour faciliter le suivi conformité.
- Déploiement léger (binaire Go) compatible avec MCP StreamableHTTP et conteneur Docker.

**Métriques qualité attendues**
- Taux de réussite des outils ≥ 99 % (hors erreurs de schéma client).
- Latence p95 du handler ≤ 1,5 s pour des configurations ≤ 200 KB.
- Taux d'erreurs HTTP 5xx ≤ 0,5 % sur 1 000 appels.

**Limites connues**
- Pas de compression automatique : les configurations volumineuses augmentent la latence et l'espace disque.
- Aucune validation métier du JSON ECharts ; une option invalide produit une page vide ou un script client en erreur.
- Idempotence absente : chaque appel crée un fichier distinct ; prévoir un mécanisme de nettoyage externe.
- Pas de prise en charge de CDN privé pour les assets ECharts (chargés depuis `cdn.jsdelivr.net`).

**Cas d'usage exemplaires**
- Génération à la volée de dashboards exploratoires pour des analystes.
- Industrialisation d'exports visuels depuis un agent conversationnel.
- Prévisualisation de chartes ECharts lors de revues produit.

**Anti-cas (à éviter)**
- Rendu de données confidentielles sans contrôle d'accès en amont.
- Utilisation comme moteur de reporting permanent (absence de purge, pas de multi-tenancy).
- Hébergement d'applications interactives complexes (limité à un chart par page).

## Interfaces & contrat d’E/S

### Entrées (contrat MCP Tool)

**JSON Schema – `GenerateEchartsPageArguments`**
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "GenerateEchartsPageArguments",
  "type": "object",
  "required": ["title", "inputSchema"],
  "properties": {
    "title": { "type": "string", "minLength": 1, "maxLength": 120 },
    "inputSchema": { "type": "object", "description": "Configuration ECharts conforme à https://echarts.apache.org" },
    "width": { "type": "number", "exclusiveMinimum": 0, "default": 1000 },
    "height": { "type": "number", "exclusiveMinimum": 0, "default": 600 }
  },
  "additionalProperties": false
}
```

**Exemple d’appel MCP (`call_tool`)**
```json
{
  "name": "generate_echarts_page",
  "arguments": {
    "title": "Taux de conversion Q4",
    "inputSchema": {
      "xAxis": { "type": "category", "data": ["Oct", "Nov", "Dec"] },
      "yAxis": { "type": "value" },
      "series": [{ "type": "line", "data": [0.12, 0.18, 0.22] }]
    },
    "width": 960,
    "height": 540
  }
}
```

### Sorties

**JSON Schema – `GenerateEchartsPageResult`**
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "GenerateEchartsPageResult",
  "type": "object",
  "required": ["content"],
  "properties": {
    "isError": { "type": "boolean", "default": false },
    "content": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["type", "text"],
        "properties": {
          "type": { "const": "text" },
          "text": { "type": "string", "format": "uri" }
        }
      },
      "minItems": 1,
      "maxItems": 1
    }
  },
  "additionalProperties": false
}
```

**Exemple de réponse**
```json
{
  "content": [
    {
      "type": "text",
      "text": "http://localhost:8989/charts/echarts_1735113987123456000.html"
    }
  ]
}
```

**Exemple d’erreur**
```json
{
  "isError": true,
  "content": [
    { "type": "text", "text": "The 'inputSchema' parameter must be a JSON object" }
  ]
}
```

### API / Tools internes
| Tool | But | Signature | Timeout recommandé | Idempotent | Notes |
| ---- | --- | --------- | -----------------: | :--------: | ----- |
| `generate_echarts_page` | Générer un HTML ECharts et renvoyer l’URL | `(title: string, inputSchema: object, width?: number, height?: number) -> ToolResult<text>` | 10 s | ❌ | Crée un fichier horodaté dans `STATIC_DIR/charts`.

### Prompts système / rôle
- Aucun prompt statique : l’agent agit comme fournisseur d’outil MCP. Les politiques conversationnelles doivent être gérées côté orchestrateur.

## Outils, ressources et permissions

| Ressource | Description | Permissions requises | Politique d’utilisation |
| --------- | ----------- | -------------------- | ----------------------- |
| FS local (`STATIC_DIR`) | Stockage des pages générées | Lecture/écriture sur le volume monté | Nettoyage externe recommandé (cron `find static/charts -mtime +7 -delete`). |
| ECharts CDN (`cdn.jsdelivr.net`) | Scripts JS chargés côté client | Accès HTTP sortant du navigateur final (pas du serveur) | Vérifier la conformité réseau si déploiement hors Internet public. |

**Politique d’appels tools**
- Un seul tool disponible ; les orchestrateurs doivent vérifier les arguments avant appel.
- Recommandation de retries côté client (max 2, backoff exponentiel 500 ms) en cas d’erreur réseau 5xx.
- Circuit breaker côté orchestrateur : ouvrir après 5 erreurs consécutives sur 1 min.

**Accès données**
- Pas de base de données ni de RAG. Les fichiers HTML contiennent la configuration fournie ; aucun contenu n’est rediffusé vers des tiers.
- Rétention : dépend du cycle de vie du volume `STATIC_DIR`. Mettre en place une purge automatique pour rester conforme au RGPD.

## Sécurité, sûreté & conformité

**Cadres de référence** : RGPD + politique interne sécurité nicho2.

**Contrôles avant appel**
- Validation stricte des types dans [`GenerateEchartsPage`](./main.go) ; rejet si `title` ou `inputSchema` mal typés.
- Limiter `width`/`height` dans l’orchestrateur (ex. `<= 1920`) pour éviter des charges mémoire.
- Filtrage amont des données sensibles : ne pas fournir de PII sans consentement documenté.

**Contrôles pendant l’exécution**
- Logs JSON (`logrus`) incluant niveau et message, sans PII (masquage côté orchestrateur si nécessaire).
- Fichiers nommés via timestamp => pas d’injection de chemin.
- Sandbox implicite : le serveur ne permet ni exécution de code ni accès réseau sortant.

**Contrôles après appel**
- Journalisation des erreurs dans stdout (redirigée vers la stack observabilité).
- Aucune persistance de requêtes dans la mémoire longue durée ; purger périodiquement les fichiers HTML.
- Rotation des logs via l’infrastructure d’hébergement (Docker/Kubernetes).

**Garde-fous recommandés**
- Content filtering côté LLM pour éviter l’injection de scripts malicieux dans `inputSchema` (l’agent injecte le JSON dans le DOM via `template.JS`).
- Allow-list des hôtes clients autorisés à exposer le serveur.
- Vérifier que l’image Docker ne tourne pas en root (utiliser `USER nobody` via override).

**Notes d’usage**
- Pas d’exécution de code non auditée : seules les configurations ECharts sont acceptées.
- Navigation web non autorisée par le serveur (pas de client HTTP sortant).
- Conformité EU CRA : garder un inventaire des dépendances (voir `go.mod`) et appliquer les mises à jour de sécurité.

## Qualité & évaluation

| Type de test | Couverture | Commande | Critère d’acceptation |
| ------------ | ---------- | -------- | --------------------- |
| Tests unitaires Go (futur) | Validation du handler | `go test ./...` | 100 % de réussite. |
| Tests scénarisés MCP | Parcours end-to-end via client MCP | `./scripts/e2e_call.sh` (à créer) | URL accessible et page charge sans erreur JS. |
| Scan statique | `gosec` sur `main.go` | `gosec ./...` | 0 vulnérabilité haute. |

**Évaluation online**
- Latence p50/p95 mesurée via métriques HTTP (exporter via reverse-proxy ou sidecar).
- Taux d’échec outil (`isError=true`) ≤ 1 %.
- Surveillance des logs navigateur pour détecter les erreurs ECharts (collecte via feedback utilisateur).

**Reproduction locale**
1. `cp .env.example .env` puis ajuster les variables.
2. `go run main.go`.
3. Utiliser un client MCP (ex. [config exemple](./README.md#%E5%AE%A2%E6%88%B7%E7%AB%AF%E9%85%8D%E7%BD%AE)) pour appeler le tool avec la payload d’exemple.

## Configuration & déploiement

### Variables d’environnement
| Variable | Description | Obligatoire | Valeur par défaut | Stockage recommandé |
| -------- | ----------- | ----------: | ----------------- | ------------------- |
| `PORT` | Port HTTP unique exposant MCP + statiques | Oui | `8989` | Secret manager ou config map. |
| `PUBLIC_URL` | URL publique utilisée dans les réponses | Non | `http://localhost:8989` | Config map, synchronisée avec ingress. |
| `LOG_LEVEL` | Niveau de logs `logrus` (`info`, `debug`, …) | Non | `info` | Config map. |
| `STATIC_DIR` | Répertoire local pour les HTML générés | Non | `static` | Volume persistant chiffré. |

### Démarrage local
- `go run main.go` (Go ≥ 1.24).
- Via Docker : `docker build -t mcp-server-echart .` puis `docker run -p 8989:8989 --rm mcp-server-echart` (adapter `PUBLIC_URL`).

### Déploiement cible
- **Local / Docker / Kubernetes** : prévoir un volume persistant monté sur `STATIC_DIR`.
- CI/CD suggérée : pipeline Go (lint, tests, build binaire, build image) + publication sur registre interne.
- Healthcheck HTTP : `/` (statique) et `/mcp/health` (à implémenter si nécessaire via middleware).

### Observabilité
- Logs JSON sur stdout (collectés par Fluent Bit / Loki).
- Exposer métriques via proxy (ex. Nginx Ingress avec `log_format` pour calcul p50/p95).
- Ajouter traçage applicatif en instrumentant `GenerateEchartsPage` (OpenTelemetry Go recommandé).

### Matrice de compatibilité
| Composant | Version supportée | Notes |
| --------- | ----------------- | ----- |
| Go | 1.24.x | cf. [`go.mod`](./go.mod). |
| `github.com/mark3labs/mcp-go` | 0.33.x | Assure compatibilité StreamableHTTP. |
| ECharts | 5.4.3 (CDN) | Chargé côté client ; vérifier la connectivité externe. |

## Gouvernance & versioning

- **Version courante** : `1.0.0` (déclarée dans `server.NewMCPServer`).
- **Schéma de versioning** : SemVer `MAJOR.MINOR.PATCH` aligné sur les évolutions du contrat d’E/S.
- **Processus de changement** :
  1. Conception + revue technique (backend + MLOps).
  2. Revue sécurité & conformité (SecOps + DPO) pour toute modification touchant aux données.
  3. Tests & validations décrits ci-dessus avant merge.

**Changelog**
- `1.0.0` : première version bêta, expose `generate_echarts_page`, stockage statique local.

## Runbook (exploitation)

### Incident / crash
1. Vérifier l’état du service (`docker ps`, `kubectl get pods`).
2. Consulter les logs (`docker logs`, `kubectl logs`, ou stack centralisée) pour identifier l’erreur Go.
3. Si blocage disque (`ENOSPC`), purger `STATIC_DIR/charts` ou augmenter le volume.
4. Redémarrer le service (`docker restart` / `kubectl rollout restart`).
5. Prévenir le support produit si impact utilisateur > 15 min.

### Dégradations acceptables
- Temps de réponse temporairement > 2 s pendant une montée de charge (< 30 min).
- Taux d’erreur outil ≤ 3 % pendant une opération de maintenance planifiée.

### Procédure de rollback
1. Restaurer l’image Docker précédente depuis le registre (`docker pull ...:previous`).
2. Redéployer (`kubectl set image deployment/mcp-server-echart ...`).
3. Vérifier santé et métriques ; purger les fichiers générés durant l’incident si inconsistants.

### Check-list diagnostic
| Étape | Commande / Action | Attendu |
| ----- | ----------------- | ------- |
| 1 | `curl -sf http://<host>:<port>/` | HTTP 200, page statique listant les fichiers. |
| 2 | `curl -sf -X POST http://<host>:<port>/mcp` (ping) | Connexion acceptée (handshake MCP). |
| 3 | `ls -lh STATIC_DIR/charts` | Fichiers horodatés, taille cohérente. |
| 4 | Inspecter logs JSON | Absence d’erreurs répétées. |
| 5 | Tester une réponse via client MCP | URL fonctionnelle et page rendue dans le navigateur. |

### Contacts & ownership
- **Dev propriétaire** : Équipe Backend Visualisation (backend@nicho2.example).
- **Exploitation** : Équipe DevOps Platform (devops@nicho2.example).
- **Sécurité** : SecOps (secops@nicho2.example).
- **Escalade** : Directeur Technique (> 24 h d’indisponibilité).

## Annexes

- **Glossaire**
  - **MCP** : Model Context Protocol, protocole outil LLM (cf. [docs/mcp.md](./README.md)).
  - **StreamableHTTP** : transport HTTP temps réel utilisé par `mcp-go`.
  - **ECharts** : bibliothèque de visualisation JS.
- **Références internes**
  - Dockerfile : [`Dockerfile`](./Dockerfile).
  - Exemple de configuration client : [`README.md`](./README.md#%E5%AE%A2%E6%88%B7%E7%AB%AF%E9%85%8D%E7%BD%AE).
- **Améliorations futures**
  - Ajouter un endpoint `/healthz` et métriques Prometheus.
  - Implémenter une politique de purge automatique configurable.
