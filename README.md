<p align="center">
  <img src="brand/nook-logo.png" alt="Nook" width="440">
</p>

**Nook** è un editor di note markdown da terminale, scritto in Go con [Bubble Tea](https://github.com/charmbracelet/bubbletea). Le note vivono in un piccolo database SQLite locale, si organizzano in cartelle e si scrivono con una **Focus Line**: la riga che stai scrivendo resta sempre al centro dello schermo, in markdown grezzo, mentre tutto il resto del documento scorre attorno a lei già renderizzato (titoli, grassetti, link, citazioni, liste e checkbox senza i marcatori).

## Perché Nook

- **Scrivi senza distrazioni.** Il cursore non si muove: si muove il testo. L'occhio resta sempre allo stesso punto.
- **Markdown vero, visto bene.** Solo la riga in focus mostra il testo grezzo; le altre sono rese con stili colorati e una palette attenuata fissa.
- **Un menù per le note.** Lista centrata sulla voce selezionata, cartelle annidate, cestino con ripristino, filtro per titolo, rinomina e creazione al volo.
- **Tastiera e mouse.** Ogni azione ha una scorciatoia; click, doppio click, triplo click e trascinamento funzionano come ci si aspetta.
- **Niente di fragile.** Le note stanno in SQLite, l'eliminazione è soft: una nota cestinata si recupera finché non la elimini per sempre.

## Installazione

Serve Go 1.26 o successivo.

```sh
git clone <url-del-repository> nook
cd nook
go build -o nook .
```

Con il `Makefile`: `make build`, `make run`, `make test`, `make vet`, `make fmt`.

## Uso

```sh
./nook
```

All'avvio si apre la lista delle note; da lì si crea, si apre, si rinomina e si
cestina. Lo storage è SQLite in `~/.mdedit/notes.db` (override con `MDEDIT_DB`;
il nome della cartella e della variabile è quello storico del progetto, prima
che si chiamasse Nook), con eliminazione soft: le note cestinate restano
recuperabili dal cestino.

L'interfaccia ha margini di 2 colonne e 1 riga su tutti i lati (azzerati sotto le
30 colonne o le 10 righe) e un footer con i tasti principali; ogni schermata ha
il suo popup di aiuto con la mappa completa.

## Lista delle note

Le voci stanno in una card larga al massimo 60 colonne, centrata orizzontalmente
e verticalmente sulla voce selezionata. Ogni voce occupa due righe: il nome e,
sotto, un dettaglio (`data · N parole` per le note, `N note` o `vuota` per le
cartelle). Ogni voce è divisa dalla successiva da una linea `─`. Il titolo in
alto è `NOTE`, con il percorso delle cartelle quando si scende (`NOTE › progetti`),
oppure `CESTINO` nella vista del cestino.

### Cartelle

Le cartelle stanno in cima alla lista (nome con il suffisso `/`, colore proprio) e
possono essere annidate. Le note si creano nella cartella in cui ci si trova.
Cestinare una cartella porta nel cestino anche le sue sottocartelle e tutte le
note che contengono; una nota ripristinata dalla cartella cestinata torna alla
radice.

| Tasto | Azione |
|---|---|
| `↑` / `↓` | muovi la selezione (resta centrata) |
| `invio` / `→` | apri la nota, oppure entra nella cartella |
| `←` / `esc` | risali alla cartella superiore |
| `c` / `ctrl+k` | nuova cartella |
| `n` / `ctrl+n` | nuova nota (nella cartella corrente) |
| `r` / `ctrl+r` | rinomina |
| `d` / `ctrl+d` | cestina (soft delete) |
| `o` / `ctrl+o` | commuta lista ↔ cestino |
| `u` | ripristina (solo nel cestino) |
| `x` / `ctrl+x` | elimina per sempre (solo nel cestino) |
| `f` / `ctrl+f` | filtro per titolo |
| `?` | aiuto |
| `q` / `ctrl+q` / `ctrl+c` | esci |

Nei popup di input (`n`, `r`, `f`) `invio` conferma ed `esc` annulla. Nei popup di
conferma (`d`, `x`) `s` o `invio` confermano, `n` o `esc` annullano.

Mouse: un click seleziona la voce, un doppio click apre la nota o entra nella
cartella.

## Editor

La **Focus Line** resta sempre alla riga centrale: muovendoti è il testo a
scorrere, non il cursore. La Focus Line mostra il markdown **grezzo**; tutte le
altre righe sono renderizzate, con marcatori nascosti (`#`, `**`, `[]()`) e stili
colorati per grassetto, corsivo, codice, link, barrato, citazioni, elenchi e
checkbox. Le citazioni (`> testo`) sono in corsivo e affiancate da una barra
verticale `▌` che corre lungo tutto il blocco, righe consecutive comprese — anche
quella della Focus Line, che mostra il `>` grezzo ma mantiene la barra. Le righe
non in focus hanno una palette attenuata, **fissa** e uguale
per tutte: l'attenuazione non dipende dalla distanza dalla Focus Line. Le righe
lunghe vengono troncate con `…`.

| Tasto | Azione |
|---|---|
| `F2` / `ctrl+s` | salva (toast di conferma) |
| `esc` / `ctrl+q` | torna alla lista; con modifiche non salvate apre il popup *Salva / Scarta / Annulla* |
| `ctrl+c` | esci subito |
| `↑` `↓` `←` `→` | movimento |
| `home` / `end` | inizio / fine riga |
| `ctrl+home` / `ctrl+end`, `ctrl+↑` / `ctrl+↓` | inizio / fine documento |
| `pgup` / `pgdn` | pagina |
| `ctrl+←` / `ctrl+→` (o `alt+←` / `alt+→`) | parola indietro / avanti |
| `shift+frecce`, `shift+home`/`end`, `shift+ctrl+←/→` | selezione |
| trascinamento col mouse | selezione dal punto premuto alla cella raggiunta |
| `backspace` / `delete` / `invio` | cancella / cancella avanti / nuova riga |
| `alt+backspace` | cancella la parola precedente |
| `ctrl+u` / `ctrl+k` | cancella fino a inizio / fine riga |

Nel popup *Salva / Scarta / Annulla*: `←`/`→` scelgono, `invio` conferma, `esc`
torna all'editor.

La selezione è visibile: il testo selezionato è evidenziato sia sulla Focus Line
(grezza) sia sulle righe rese, dove l'evidenziazione viene mappata sui segmenti
markdown. Scrivendo o cancellando, il testo selezionato viene sostituito; le
azioni di formattazione lo avvolgono.

Mouse: un click porta la riga cliccata a Focus Line (la colonna viene mappata sul
testo grezzo attraverso i segmenti markdown), un doppio click seleziona la parola,
un triplo click la riga; tenendo premuto il tasto sinistro e trascinando si
estende la selezione. Con il mouse attivo la selezione nativa del terminale si fa
tenendo premuto `Shift` (o dal menu del terminale).

## Formattazione

`F3` apre la **palette di formattazione**: `↑`/`↓` per scorrere le azioni, `invio`
per applicare, `esc` per chiudere senza applicare.

In alternativa `ctrl+t` seguito da una lettera:

| | | | |
|---|---|---|---|
| `b` grassetto | `i` corsivo | `s` barrato | `k` codice |
| `u` lista | `x` checkbox | `l` link | `q` citazione |
| `h` titolo a ciclo (`#` → `##` → `###`) | `0` toglie il titolo | | |

Se dopo `ctrl+t` arriva un altro tasto, l'attesa viene annullata e il tasto fa il
suo mestiere. Con una selezione attiva l'azione avvolge il testo selezionato;
senza selezione sulla Focus Line il markdown si può anche scrivere a mano, dato
che lì il testo è grezzo.

## Aiuto

`F1` oppure `ctrl+/` aprono e chiudono il popup di aiuto in qualunque schermata;
`?` funziona anche dalla lista (dove non si sta scrivendo testo). Qualsiasi altro
tasto chiude il popup.

## Note sui tasti

Alcune combinazioni non sono esprimibili da un terminale: `ctrl+shift+lettera`
arriva come la stessa lettera con `ctrl`, e `ctrl+?` arriva come `backspace`. I
tasti `alt+…` richiedono un terminale che li invii (su macOS serve "Use Option as
Meta key" in Terminal.app o "Option key: Esc+" in iTerm2); dove non arrivano,
valgono gli equivalenti con `ctrl` (`ctrl+←`/`ctrl+→`).

## Diagnostica tasti

Con `MDEDIT_DEBUG_KEYS=1` ogni `tea.KeyMsg` ricevuto viene appeso in append a
`~/.mdedit/keys.log` (una riga per tasto, con `Type`, `Runes`, `Alt`, `Paste` e
`String`), utile per capire cosa invia davvero il terminale. Vengono inoltre
loggati i messaggi che l'applicazione non riconosce — per esempio le sequenze di
escape (CSI) sconosciute, che bubbletea recapita come messaggi generici — nel
formato:

```
Type=%T Val=%q
```

cioè il tipo Go del messaggio e il suo valore tra virgolette (una riga per
messaggio). I messaggi già gestiti (`WindowSizeMsg`, `KeyMsg`, `MouseMsg` e i
tick che spengono i toast) non finiscono in questo log: i tasti restano
registrati nella riga di `logKey`. Senza quella variabile (o con un valore
diverso da `1`) non viene scritto nulla e il file non viene creato.

```sh
MDEDIT_DEBUG_KEYS=1 ./nook
```

## Limitazioni del prototipo

Rendering del markdown riga per riga: i blocchi di codice recintati e le tabelle
compaiono come testo semplice, e i paragrafi non vengono rifluiti. Le righe
renderizzate sono troncate a una riga di schermo, non mandate a capo. Niente
syntax highlighting dei linguaggi di programmazione, ricerca nel testo, o undo.

## Il logo

Il logo di Nook segue lo stile di quello di [Orcrist](https://github.com/simone20a/Orcrist): la prima lettera, la **N**, è una pila di sei strati, uno per colore della palette, ciascuno arretrato rispetto a quello davanti lungo una sola direzione (in alto a sinistra, a 200°); le altre lettere sono del viola dello strato frontale, `#52389c`. Le lettere sono tracciati vettoriali, non testo, così il logo è identico ovunque. `brand/build.py` rigenera gli SVG e `brand/render.py` il PNG.

## Come è stato costruito

Nook è stato creato **esclusivamente con il coding agent [Orcrist](https://github.com/simone20a/Orcrist)**, usando soltanto i modelli **DeepSeek Flash** e **DeepSeek Pro**: nessun altro agente o modello ha scritto il codice.
