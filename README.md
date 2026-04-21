# KSP Modder

KSP Modder is a local web-based mod manager for Kerbal Space Program.

Browse and toggle mods in your `GameData` folder, detect conflicts, scan logs for issues, manage save game backups, and export your mod list — all from a browser interface.

## Features

- Enable, disable, or remove mods
- Mod notes and profiles for saving and restoring mod sets
- Log viewer with error and warning filters
- Mod error scanning that groups log entries by mod
- Export your mod list to a `.txt` file for sharing or troubleshooting

---

## Setup

1. Download the binary for your system.
2. Place the binary in a folder where you want to run it.
3. Launch the binary.

## Usage

Open the web app in your browser and use the interface to manage mods and browse save files.

A config file will be created and stored in your OS's application data folder. It stores settings such as:

- Accent color
- Game install location
- UI preferences

---

## Notes

- This is not a CKAN replacement — it's designed to be used alongside manual mod installation or tools like CKAN.
- Conflict detection is basic and does not guarantee full compatibility checks. *

> \* edit — it does not work at all

---

## Support

<details>
<summary>Application data storage</summary>

Your config file is stored in your OS's default application data location, inside a folder called `ksp-modder`:

| OS | Path |
|---|---|
| macOS | `~/Library/Application Support/ksp-modder` |
| Windows | `C:\Users\<username>\AppData\Roaming\ksp-modder` |
| Ubuntu / Debian / Mint | `~/.config/ksp-modder` |

To reset settings, delete that folder.

</details>

<details>
<summary>Troubleshooting</summary>

### App won't launch

**macOS — binary opens as a file instead of running**

This usually means the file isn't marked as executable. Navigate to the binary's directory in Terminal and run:

```bash
chmod +x <name_of_program>
```

> sorry these instructions are not as clean as I'd like... but I hope and pray you don't need them

### More Trouble?..
Please email: matthew@orboul.com

</details>