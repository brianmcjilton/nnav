# nnav — Notes Navigator

A fast, keyboard-driven **terminal UI (TUI)** for browsing your notes.  
`nnav` scans a directory of plain-text or Markdown (`.txt`, `.md`) files and shows them in a collapsible tree.  
The first Markdown heading (`# ...`) in each note is used as its description.

---

## ✨ Features

- Single binary, no external services
- Collapsible directory tree of your notes
- Supports `.md` and `.txt` files
- Shows the first Markdown heading as a description
- Vim-style keybindings (`h/j/k/l`, `q` to quit, etc.)
- Opens the selected note in your editor (respects `$VISUAL` → `$EDITOR`, defaults to `vim → vi → nano`)
- Config file at `~/.nnav` defines where your notes live:

    notesdir=~/notes

  Created automatically on first run if missing. Edit it to point to any directory you want.

---

## ⌨️ Keybindings

| Key            | Action                           |
|----------------|----------------------------------|
| `↑` / `k`      | Move up                          |
| `↓` / `j`      | Move down                        |
| `→` / `l`      | Expand directory                 |
| `←` / `h`      | Collapse directory               |
| `Enter`        | Open note in your editor         |
| `r`            | Reload tree (re-scan notes dir)  |
| `q` / `Esc`    | Quit                             |

---

## 📦 Installation

### Option 1: Prebuilt packages (recommended)

Visit the [Releases page](https://github.com/brianmcjilton/nnav/releases) and download the latest package for your system.


---

## 🚀 Usage

After installing, just run:

    nnav

On first run, `~/.nnav` will be created with:

    notesdir=~/notes

Edit that file to point to your own notes directory.

---

## 🛠 Roadmap

- [ ] Search/filter notes (`/`)
- [ ] Read-only view with pager (`o`)
- [ ] Configurable file extensions
- [ ] Live refresh with `fsnotify`
- [ ] Ship prebuilt `.deb` and `.rpm` via GitHub Releases

---

## 🤝 Contributing

Pull requests and issues are welcome.  
Please open an issue first if you’d like to discuss a major change.

---

## 📜 License

MIT © [Brian McJilton](https://github.com/brianmcjilton
