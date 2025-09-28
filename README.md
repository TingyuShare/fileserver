# File Server

A simple HTTP file server written in Go that supports file and folder uploads/downloads.

## Features

- Upload single files or folders (as ZIP with `.up` extension, auto-extracts)
- List files and directories via web interface
- Download files or zip directories
- Automatic unique naming to avoid conflicts
- Path traversal protection
- Cross-platform builds (Linux, macOS, Windows)

## Installation

### From Source

1. Clone the repository:
   ```
   git clone <repo-url>
   cd file-server
   ```

2. Build the binary:
   ```
   make build
   ```

   Or for all platforms:
   ```
   make build-all
   ```

3. Run the server:
   ```
   ./fileserver -dir=/path/to/serve
   ```

### From Releases

Download the pre-built binary from GitHub Releases for your platform and run it.

## Usage

- Start the server: `./fileserver -dir=./` (serves current directory on port 8080+)
- Access the web interface: http://localhost:8080
- Upload files via the form (use `.up` for folders)
- Download via links on the page

## Building for Different Platforms

Use the Makefile:

- `make build-linux`: Linux AMD64
- `make build-darwin`: macOS AMD64
- `make build-windows`: Windows AMD64
- `make build-all`: All platforms

Binaries are output to `build/` directory.

## CI/CD

GitHub Actions workflow builds binaries on PRs and creates releases on tags (e.g., `git tag v1.0.0 && git push --tags`).

## License

MIT