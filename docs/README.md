# Mediarizer 2

Mediarizer is a command-line tool for organizing your media files. It allows you to easily sort your photos and videos into folders based on date, location, file type, and other criteria.

## Installation

To install Mediarizer, you'll need to have Go installed on your system. Once you have Go installed, you can run the following command:

```go
go install github.com/keybraker/mediarizer-2@latest
```

This will download and install the Mediarizer package to your Go workspace.

## Usage

Mediarizer can be used from the command line by running the `mediarizer` command followed by various flags. Here's a list of available flags:

| Name        |          Argument           |  Default  | Description                                                                             |
| :---------- | :-------------------------: | :-------: | :-------------------------------------------------------------------------------------- |
| `-help`     |          `<none>`           | `<none>`  | Displays a usage guide of Mediarizer.                                                   |
| `-version`  |          `<none>`           | `<none>`  | Displays the current version of Mediarizer.                                             |
| `-input`    |          `<path>`           | `<none>`  | Specifies the path to the file or directory that you want to organize.                  |
| `-output`   |          `<path>`           | `<none>`  | Specifies the path to the output directory where you want to store the organized files. |
| `-unknown`  |          `<bool>`           | `<true>`  | Moves media that have no metadata to an "undetermined" folder.                          |
| `-location` |          `<bool>`           | `<false>` | Moves media according to geo-location instead of date.                                  |
| `-types`    | `<comma seperated strings>` |  `<all>`  | Organizes only given file type/s (.jpg, .png, .gif,.mp4, .avi, .mov, .mkv).             |
| `-photo`    |          `<bool>`           | `<true>`  | Organizes only photos.                                                                  |
| `-video`    |          `<bool>`           | `<true>`  | Organizes only videos.                                                                  |

To use Mediarizer, simply run the `mediarizer` command followed by any desired flags. For example:

```bash
mediarizer -input=/path/to/files -output=/path/to/organized/files -types=jpg,png,gif -location
```

This command will organize all JPEG, PNG, and GIF files in the specified input directory by geo-location and move them to the specified output directory.

## Contributing

If you'd like to contribute to Mediarizer, please fork the repository and submit a pull request. We welcome contributions of all kinds, including bug fixes, feature requests, and code improvements.

## License

Mediarizer is released under the MIT License. See [LICENSE](https://github.com/mediarizer/docs/LICENSE) for details.
