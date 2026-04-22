# preflt — Requirements
*Pilot-style CLI checklist runner · Go · Single Binary*

---

## Concept
- Pilot-style CLI checklist runner — kein To-Do App, sondern prozessualer Ablauf mit Disziplin
- Jede Checklist ist eine definierte Situation: Vor dem Deploy, Nach der Hochzeit, Sprint-Abschluss, …
- Checklisten werden als YAML-Files definiert und versioniert
- Abschluss einer Checklist kann Automationen auslösen (Webhook, Slack, Shell-Command)
- Einsetzbar überall wo Prozesse wiederholt, diszipliniert und nachvollziehbar ablaufen müssen: DevOps, Startups, Persönliches, Handwerk, Medizin, Gastronomie, Recht, Content
- **Als Guardrail-Layer für LLM-Agenten** — ein Agent der vor kritischen Aktionen eine Checklist abarbeitet ist zuverlässiger als einer der das implizit "weiß". preflt kann als strukturierter Kontrollpunkt in Agenten-Pipelines eingesetzt werden (siehe eigene Sektion)

---

## YAML Schema
- `name`, `description`, `version` als Metadaten
- `items[]` mit `id`, `label`, `response` (was bestätigt werden muss), `note`
- `type: do | check` pro Item — `do` ist eine Aktion, `check` ist eine Verifikation (kann Wert-Input erfordern)
- `type: normal | emergency` pro Checklist — emergency Items sind nie skippbar
- `na_allowed: true` — Item kann explizit als N/A bestätigt werden (nicht einfach skip, sondern bewusste Bestätigung)
- `phases[]` — Checklist kann in benannte Phasen unterteilt sein (z.B. BEFORE START, AFTER START, TAXI)
- `blocking: true` → Liste pausiert bis Item abgehakt
- `condition` → bedingte Items mit `ask` / `if_yes` / `if_no` / `skip`
- `on_complete` auf Listen-Ebene: Slack, Webhook, Shell-Command, `trigger_checklist`
- `on_complete` auf Item-Ebene: für item-spezifische Automationen
- `condition` auf Item-Ebene kann per `if_yes` / `if_no` eine andere Checklist triggern (`trigger_checklist`)
- `abort: true` bricht aktuelle Liste ab und springt direkt zur neuen (z.B. Emergency-Checklist)
- `timeout` pro Item: Warnung wenn zu lange offen
- `schedule` Block im YAML ist ein **Hinweis/Vorschlag** — er beschreibt die *Intention* der Checklist (z.B. "täglich morgens"), aber der tatsächliche User-Schedule wird separat in `~/.preflt/schedules.json` gespeichert

---

## CLI Interface
- `preflt run <name>` — Checklist per Name starten oder Resume (aus `~/.preflt/`)
- `preflt run ./path/to/checklist.yaml` — direkt per Pfad, z.B. aus einem Repo heraus
- Pfad-Variante: State/Log wird trotzdem normal unter `~/.preflt/runs/` gespeichert
- `preflt list` — alle verfügbaren Checklisten (global + lokale im cwd)
- `preflt history <name>` — vergangene Runs einer Checklist
- `preflt log <datum>` — alles an einem Tag
- `preflt new` — interaktiver YAML-Wizard
- `preflt web <n>` — Checklist im Browser starten (lokaler HTTP-Server + Browser öffnet automatisch)
- `preflt web ./path/to/checklist.yaml` — auch per Pfad
- `preflt schedule <n> --pending` — Checklist als "gehört erledigt" markieren, ohne festes Datum
- `preflt schedule <n> --from <datum>` — Checklist ab einem bestimmten Datum vorschlagen
- `preflt schedule <n> --frequency daily|weekly --on <day>` — wiederkehrenden Schedule setzen
- `preflt schedule <n> --cooldown 7d` — Mindestabstand zwischen zwei Runs
- `preflt schedule` — alle aktiven Schedules anzeigen
- `--host` Flag, default localhost — mit `0.0.0.0` im lokalen Netzwerk erreichbar (z.B. für Teammate im selben WiFi)

---

## State & Resume
- Laufender State wird automatisch gespeichert: `~/.preflt/runs/<id>/<timestamp>.json`
- Beim nächsten Start: "Laufende Session gefunden (vor 23min). Fortsetzen? [y/n]"
- State enthält: aktueller Item-Index, gecheckte Items, Zeitstempel pro Item, Skip-Begründungen
- Ctrl+C sicher — kein Datenverlust

---

## Logging & History
- Jeder abgeschlossene Run → JSON-File lokal gespeichert
- Log enthält: checklist name, started/completed timestamps, completed_by (git user / hostname), items mit checked/skipped/at
- Local-first — Sync (S3, Git) als späteres Feature, Struktur bereits sync-ready
- `preflt history` zeigt letzte Runs mit Dauer und Status
- Verkettete Runs (`run_chain`) werden als zusammenhängend geloggt — ein Trigger-Event ist im Log sichtbar

---

## Automations
- Slack: Channel + Message (mit Checklist-Kontext als Template-Variablen)
- Shell: beliebiger Shell-Command nach Abschluss
- Reihenfolge der Automations ist definiert und sequenziell
- Fehler in Automation blockt nicht — wird geloggt, Checklist gilt als abgeschlossen

### Webhooks
- **Pro Item** (`on_complete` auf Item-Ebene) — POST wenn ein einzelnes Item abgehakt wird
- **Pro Liste** (`on_complete` auf Listen-Ebene) — POST wenn die gesamte Liste abgeschlossen ist
- Webhook-Body enthält immer: checklist name, item id/label, timestamp, run-id, completed_by
- Gedacht als n8n-Eingang — n8n übernimmt dann Notifications, weitere Automationen, etc.
- Webhook als Trigger zum *Starten* einer Checklist — explizit out of scope für v1

---

## Checklist Chaining
- Eine Checklist kann eine andere auslösen — per Item-Condition oder nach Abschluss
- Item-level: `if_no: trigger_checklist: engine-failure` + optional `abort: true`
- Listen-level: `on_complete: trigger_checklist: post-deploy-monitoring`
- `abort: true` — aktuelle Liste wird als abgebrochen markiert, neue startet sofort
- Ohne `abort` — aktuelle Liste läuft zu Ende, dann startet die nächste
- Chains werden im Log als `run_chain` zusammengefasst:

```json
{
  "run_chain": [
    { "checklist": "pre-deploy", "status": "completed" },
    { "checklist": "post-deploy-monitoring", "status": "completed" }
  ]
}
```

- Schutz gegen Loops: gleiche Checklist kann nicht zweimal in einer Chain vorkommen

---

## Distribution
- Go → single binary, kein Runtime nötig
- `bubbletea` für Terminal-UI
- `goreleaser` → automatische Binaries für Mac/Linux/Windows via GitHub Releases
- Ziel: `brew install` oder curl-one-liner, fertig
- Kein Node, kein Python, kein Install-Stress für Teammates

---

## Beispiel-Checklisten
- `pre-deploy` — Tests, Staging, Changelog, Slack-Notification nach Abschluss
- `neuer-parkplatz-onboarding` — Vertrag, Fotos, Adresse, erste Buchung
- `sprint-abschluss` — Linear closed, Retro, Changelog
- `neuer-mitarbeiter` — Accounts, Slack, Linear, erster 1:1
- `incident-response` — Identifiziert, informiert, fixed, post-mortem
- `vendor-meeting-prep` — Agenda, Fragen, Aufnahme ok?
- `vor-der-reise` — Ladekabel, Dokumente, Hotel, Bescheid geben
- `nach-der-reise` — Koffer ausgepackt, Spesen eingereicht, Pflanzengegossen, Wäsche
- `wochenbeginn` — Kalender, Priorities, Inbox
- `plants` — wöchentlich, jede Pflanze als Item: Wasser, Dünger, Erde checken
- `morning-routine` — täglich, Vitamine, Wasser, etc.
- `jeden-abend` — täglich abends, Vorbereitung nächster Tag

---

## preflt als LLM-Guardrail
LLMs machen Fehler durch fehlende Struktur, nicht durch fehlende Intelligenz. Ein Agent der vor kritischen Aktionen eine Checklist abarbeitet ist zuverlässiger als einer der das implizit "weiß" — genau wie Piloten in einem hochkomplexen System das theoretisch alles selbst könnte.

### Konzept
- preflt kann als **strukturierter Kontrollpunkt in Agenten-Pipelines** eingesetzt werden
- Vor einer kritischen Aktion (API-Call, Datenbankschreibung, E-Mail-Versand) führt der Agent `preflt run pre-action-check` aus
- Erst wenn die Checklist `completed` zurückgibt läuft die Aktion weiter
- Der Run-Log ist gleichzeitig Audit-Trail: was wurde geprüft, wann, von welchem Agenten

### Beispiel-Flow
```
Agent will E-Mail an Kundenliste senden
  → preflt run email-blast-check
      ✓ Empfängerliste validiert
      ✓ Unsubscribes gefiltert
      ✓ Staging-Versand erfolgreich
      ✓ Betreffzeile reviewed
  → Status: completed → Agent sendet
```

### Interface-Idee (later)
- `preflt run <name> --json` — maschinenlesbarer Output für Agenten
- Exit-Code 0 = completed, 1 = aborted/incomplete — einfach in Shell-Pipelines integrierbar
- Checklist-Items könnten `automated: true` haben — Agent bestätigt selbst, Mensch bestätigt manuell
- Explizit out of scope für v0.x — aber Schema von Anfang an kompatibel halten

---

## Web UI
- `preflt web <n>` startet einen lokalen HTTP-Server und öffnet den Browser automatisch
- Gleichberechtigter Mode neben CLI — gleicher State darunter, gleiche Resume-Logik, gleiche Automations bei Completion
- HTML/CSS/JS als `embed.FS` in die binary eingebettet — kein extra Dependency, alles in der single binary
- Standard: `localhost:8080`, nur lokal erreichbar
- Mit `--host 0.0.0.0` im lokalen Netzwerk erreichbar (Teammate im selben WiFi kann mitmachen)
- Keyboard shortcuts im Browser: `Enter` / `Space` bestätigen, `n` für N/A, `y`/`n` für Condition-Items
- Echtzeit-Sync zwischen CLI und Web wenn beide offen: TBD / später
- Für nicht-technische Teammates die kein Terminal mögen

---

## Scheduled Checklists
- Scheduling ist **Nutzer-Intent, nicht Template-Definition** — wann eine Checklist fällig ist gehört nicht ins YAML sondern in den lokalen User-State (`~/.preflt/schedules.json`)
- Das Template kann einen `schedule`-Block als *Hinweis* mitbringen (z.B. `frequency: daily`) — dieser wird beim ersten `preflt schedule`-Aufruf als Vorschlag angeboten, aber nie erzwungen
- Kein Daemon, kein Hintergrundprozess — Vorschläge passieren nur beim Aufruf von `preflt`
- "Bereits erledigt" wird aus dem Log gecheckt — existiert ein completed Run von heute, wird nicht nochmal vorgeschlagen

### Scheduling-Modi

| Modus | Kommando | Verhalten |
|---|---|---|
| Offen / pending | `preflt schedule nach-der-reise --pending` | Taucht in der Pending-Liste auf bis manuell erledigt oder dismissed |
| Ab Datum | `preflt schedule nach-der-reise --from 2024-06-15` | Wird ab diesem Datum in der Pending-Liste vorgeschlagen |
| Wiederkehrend täglich | `preflt schedule morning-routine --frequency daily` | Jeden Tag vorgeschlagen |
| Wiederkehrend wöchentlich | `preflt schedule wochenabschluss --frequency weekly --on sunday` | Jeden Sonntag |
| Mit Tageszeit-Hint | `preflt schedule morning-routine --period morning` | Kein hard enforcement, nur Kontext im Startup-Screen |
| Cooldown | `preflt schedule plants --cooldown 7d` | Nicht öfter als einmal in N Tagen |

### Startup-Screen wenn fällige Listen vorhanden

```
Guten Morgen, Benni.
📋 Fällige Checklisten:
  → Morgenroutine       (heute noch nicht gemacht)
  → Wochenabschluss     (Sonntag, noch offen)
  → Nach der Reise      (offen seit 3 Tagen)
Was möchtest du starten? [1/2/3/skip]
```

---

## Piloten-Prinzipien
- **Challenge & Response** — bewusstes Vorlesen + bewusstes Bestätigen, nicht einfach durchklicken. `--crew` Mode (zwei Terminals) als späteres Feature, Prinzip gilt aber immer
- **Normal vs Emergency** — `type: normal | emergency` im YAML. Emergency-Checklisten sind nie skippbar, keine Items können als N/A markiert werden
- **Do vs Check** — `type: do` ist eine Aktion die ausgeführt wird, `type: check` ist eine Verifikation (kann konkreten Wert-Input erfordern, z.B. "Treibstoff: ___")
- **Phasen** — Checklist in benannte Abschnitte unterteilt. Phase abschließen, später bei nächster Phase fortsetzen. Granulareres Resume als nur Item-Index
- **N/A bewusst bestätigen** — nicht einfach skip, sondern explizite Bestätigung "nicht relevant" — wird im Log als `na` markiert, nicht als `skipped`
- **Kein Zurück** — Navigation ist vorwärts-only. Kein Scroll zurück zu einem früheren Item. Fehler → separater Korrektur-Flow (TBD), nicht rückwärts navigieren

---

## Offen / TBD
- Sync der Logs (S3, Git-Repo, eigener Server) — später
- Multi-User / Team-shared Run-State — später
- Web-UI für History — vielleicht nie (CLI first)
- Checklist-Abhängigkeiten (A muss vor B) — vielleicht

---

## Checklist Registry (Idee)
- Öffentliche, community-gepflegte Sammlung von Checklisten — ähnlich Homebrew Formulas oder Helm Charts
- `preflt install pre-deploy` zieht das YAML von einem zentralen GitHub-Repo
- `preflt install github.com/user/repo/checklist.yaml` — direkt aus beliebigem Repo
- Registry-Repo: `github.com/preflt/registry` — kuratierte, geprüfte Checklisten
- YAML-Schema von Anfang an so designt dass Registry-Kompatibilität gewährleistet ist
- Aus einem CLI-Tool wird eine kleine Plattform — aber erst wenn der Kern steht
- Explizit out of scope für v0.x

---

## MVP — v0.1
*Radikaler Schnitt: Was brauche ich damit ich morgen meine erste Checklist durchführen kann?*

### Im Scope

| Feature | Details |
|---|---|
| `preflt run <name\|path>` | Kernfunktion — per Name (aus `~/.preflt/`) oder direktem Pfad |
| YAML parsen | `items[]`, `phases[]`, `type: do\|check`, `na_allowed`, `note` |
| Bubbletea TUI | Item anzeigen, bestätigen, N/A markieren, Phasen-Fortschritt |
| State + Resume | Automatisches Speichern, Ctrl+C-sicher, "Fortsetzen?"-Prompt beim nächsten Start |
| Run-Log als JSON | Basis für alles weitere — `completed_by`, Zeitstempel pro Item, Status |
| `preflt list` | Alle verfügbaren Checklisten (global + lokal im cwd) |
| `preflt history <name>` | Letzte Runs einer Checklist mit Dauer und Status |

### Bewusst raus aus v0.1

- Web UI → v0.3
- Automations, Webhooks, Slack → v0.2
- Checklist Chaining + Conditions → v0.2
- Scheduling → v0.4 (proper user-intent scheduling via `preflt schedule`)
- `preflt new` Wizard → vielleicht nie
- `--crew` Mode → v1.0
- Registry / `preflt install` → nach v1.0
