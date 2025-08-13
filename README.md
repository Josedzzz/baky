# Baky - Simple Backup Wrapper for File Editing

## About

Is a lightweight shell tool that automatically creates a backup of a file before you edit it.
It’s perfect for sysadmins, developers, or anyone who frequently modifies configuration files and wants a quick safety net.

### How it works

- You run baky with your text editor.
- baky makes a timestamped copy of your file in a backups folder inside the current directory.
- It then opens the file in your chosen editor.

### Example

```bash
baky vim /etc/shorewall/shorewall.conf
```

This will:

- Create ./backups/shorewall.conf_YYYYMMDD_HHMMSS
- Open /etc/shorewall/shorewall.conf in vim

## Installation

### Linux / MacOS

1. Download the script:

```bash
curl -LO https://raw.githubusercontent.com/Josedzzz/baky/main/main.sh
```

2. Make it executable:

```bash
chmod +x main.sh
```

3. (Optional) Move it to your PATH so you can run baky from anywhere:

```bash
mv main.sh baky
sudo mv baky /usr/local/bin/
```

Note: On Apple Silicon Macs, replace /usr/local/bin with /opt/homebrew/bin

4. Usage (param1: text editor, param2: file)

```bash
baky vim myfile.txt
```

## Considerations

Since the project is still in early development, there may be bugs or areas where things could be done better. Suggestions, issues, and contributions are more than welcome!

## Contributions

Contributions are welcome! Feel free to fork this repo, submit pull requests, or open issues for bugs and improvements.

## <3

Developed with love for learning.
