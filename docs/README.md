# Mediarizer 2

Mediarizer2 is a command-line tool for organizing your media files.
It allows you to easily sort your photos and videos into folders based on date, location, file type, and other criteria.

> As speed is prioritized instead of copying a file it gets transferred to a different location; this means the input and output folders shall be on the same physical drive in order to assure maximum performance.

## Installation

1. To install Mediarizer2, you'll need to have Go installed on your system.
Once you have Go installed, you can run the following command:

    ```bash
    go install github.com/keybraker/mediarizer-2@latest
    ```

    This will download and install the Mediarizer2 package to your Go workspace.

1. If you wish to build and execute Mediarizer2 simply run the following build command in the repo directory:

    unix:

    ```bash
    go build -o mediarizer2 ./app
    ```

    windows:

    ```bash
    go build -o mediarizer2.exe .\app
    ```

## Execution

unix:

```bash
./mediarizer2 -input=/path/to/files -output=/path/to/organized/files
```

windows:

```bash
.\mediarizer2.exe -input=/path/to/files -output=/path/to/organized/files
```

## Usage

Mediarizer2 can be used from the command line by running the `mediarizer2` command followed by various flags.

Here's a list of available flags:

| Name         |          Argument           |  Default  | Description                                                                            | Mandatory |
| :----------- | :-------------------------: | :-------: | :------------------------------------------------------------------------------------- | :-------: |
| `input`      |          `<string>`         |    `-`    | Path to source file or directory                                                       |   true    |
| `output`     |          `<string>`         |    `-`    | Path to destination directory                                                          |   true    |
| `unknown`    |          `<bool>`           | `<true>`  | Move files with no metadata to undetermined folder                                     |   false   |
| `duplicate`  |         `<string>`          | `<move>`  | Duplication handling, default "move " (move, skip, delete)                             |   false   |
| `location`   |          `<bool>`           | `<false>` | Organize files based on their geo location                                             |   false   |
| `types`      | `<comma separated strings>` |  `<all>`  | Comma separated file extensions to organize (.jpg, .png, .gif, .mp4, .avi, .mov, .mkv) |   false   |
| `photo`      |          `<bool>`           | `<true>`  | Only organise photos                                                                   |   false   |
| `video`      |          `<bool>`           | `<true>`  | Only organise videos                                                                   |   false   |
| `format`     |         `<string>`          | `<word>`  | Naming format for month folders, default "word" (word, number, combined)               |   false   |
| `help`       |             `-`             |    `-`    | Display usage guide                                                                    |   false   |
| `verbose`    |          `<bool>`           | `<false>` | Display progress information in console                                                |   false   |
| `version`    |             `-`             |    `-`    | Display version information                                                            |   false   |

## Contributing

If you'd like to contribute to Mediarizer 2, please fork the repository and submit a pull request. We welcome contributions of all kinds, including bug fixes, feature requests, and code improvements.

## License

Mediarizer is released under the MIT License. See [LICENSE](https://github.com/mediarizer/docs/LICENSE) for details.
