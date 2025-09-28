package main

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// uploadDir 定义上传文件的存储目录

var uploadDir string

// main 函数启动 HTTP 服务器
func main() {
	flag.StringVar(&uploadDir, "dir", ".", "Directory to serve files")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	log.Printf("Serving directory: %s", uploadDir)

	// 创建上传目录，如果不存在
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatal(err)
	}

	// 注册处理函数
	http.HandleFunc("/", listHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/download", downloadHandler)

	port := 8080
	for {
		addr := fmt.Sprintf(":%d", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			if strings.Contains(err.Error(), "bind") || strings.Contains(err.Error(), "address already in use") {
				port++
				continue
			}
			log.Fatal(err)
		}
		log.Printf("Server starting on %s", addr)
		log.Fatal(http.Serve(ln, nil))
	}
}

// uploadHandler 处理文件上传请求
// 支持单个文件或 .up 文件（用于文件夹上传，内容为ZIP）
// 使用 POST 方法，表单字段名为 "file"
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析 multipart 表单，最大 32MB
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file in form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := header.Filename
	if filename == "" {
		http.Error(w, "Empty filename", http.StatusBadRequest)
		return
	}

	log.Printf("Uploading file: %s", filename)

	// 安全路径：防止路径遍历
	baseName := filepath.Base(filename)
	ext := filepath.Ext(baseName)

	// 生成唯一文件名
	safeName := generateUniqueName(uploadDir, baseName, ext)
	log.Printf("Generated safe name: %s", safeName)

	// 如果是 .up 文件（文件夹上传，内容为ZIP），解压到子目录
	if strings.ToLower(ext) == ".up" {
		// 创建临时 ZIP 文件在系统临时目录
		tempZip := filepath.Join(os.TempDir(), "temp_upload.zip")
		log.Printf("Creating temp ZIP for folder: %s", tempZip)
		dst, err := os.Create(tempZip)
		if err != nil {
			log.Printf("Error creating temp ZIP: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		defer os.Remove(tempZip) // 清理临时文件

		if _, err := io.Copy(dst, file); err != nil {
			log.Printf("Error copying to temp ZIP: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 解压 ZIP 到子目录（使用唯一名称，去掉 .up）
		folderName := strings.TrimSuffix(safeName, ".up")
		extractDir := filepath.Join(uploadDir, folderName)

		// 如果目录已存在，生成带 6 位 hash 后缀的名称
		for {
			if _, err := os.Stat(extractDir); os.IsNotExist(err) {
				break
			}
			log.Printf("Directory %s exists, generating hash suffix", extractDir)
			hashSuffix := generateHashSuffix(folderName)
			folderName = folderName + "_" + hashSuffix
			extractDir = filepath.Join(uploadDir, folderName)
		}

		log.Printf("Extracting folder ZIP to directory: %s", extractDir)
		if err := extractZip(tempZip, extractDir); err != nil {
			log.Printf("Error extracting ZIP: %v", err)
			http.Error(w, "Failed to extract folder ZIP", http.StatusInternalServerError)
			return
		}

		log.Printf("Folder extracted successfully to %s", extractDir)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// 普通文件：直接保存（包括 .zip 文件）
	targetPath := filepath.Join(uploadDir, safeName)
	log.Printf("Saving file to: %s", targetPath)
	dst, err := os.Create(targetPath)
	if err != nil {
		log.Printf("Error creating file: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		log.Printf("Error copying file: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("File saved successfully: %s", safeName)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// generateUniqueName 生成唯一文件名，避免同名冲突
func generateUniqueName(dir, baseName, ext string) string {
	nameWithoutExt := strings.TrimSuffix(baseName, ext)
	counter := 1
	safeName := baseName

	for {
		targetPath := filepath.Join(dir, safeName)
		log.Printf("Checking existence of: %s", targetPath)
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			log.Printf("Path %s does not exist, using %s", targetPath, safeName)
			return safeName
		}
		log.Printf("Path %s exists, trying next: %s", targetPath, safeName)
		safeName = fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext)
		counter++
	}
}

// generateHashSuffix 生成 6 位基于名称的 hash 后缀
func generateHashSuffix(name string) string {
	h := md5.New()
	h.Write([]byte(name + time.Now().Format("20060102150405"))) // 添加时间戳以增加唯一性
	hashBytes := h.Sum(nil)
	hashStr := hex.EncodeToString(hashBytes)
	return hashStr[:6]
}

// listHandler 处理根路径，显示当前目录的文件和文件夹列表
func listHandler(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		return
	}

	var sb strings.Builder
	var dirItems, fileItems []string
	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
    <title>File Manager</title>
    <meta charset="UTF-8">
</head>
<body>
    <h1>文件和文件夹管理</h1>
    <p>上传文件：直接选择文件上传。<br>上传文件夹：将文件夹压缩为 ZIP，重命名为 .up 后缀后上传（将自动解压）。</p>
    <form action="/upload" method="post" enctype="multipart/form-data">
        <input type="file" name="file" required>
        <input type="submit" value="上传">
    </form>
    <h2>当前目录内容:</h2>
    <h3>文件夹:</h3>
    <ul>`)

	for _, entry := range entries {
		name := entry.Name()
		escapedName := html.EscapeString(name)
		if entry.IsDir() {
			dirItems = append(dirItems, fmt.Sprintf(`<li><a href="/download?path=%s">%s</a> (下载为 ZIP)</li>`, url.QueryEscape(name), escapedName))
		} else {
			fileItems = append(fileItems, fmt.Sprintf(`<li><a href="/download?path=%s">%s</a></li>`, url.QueryEscape(name), escapedName))
		}
	}

	for _, dirItem := range dirItems {
		sb.WriteString(dirItem)
	}
	sb.WriteString(`</ul>
    <h3>文件:</h3>
    <ul>`)
	for _, item := range fileItems {
		sb.WriteString(item)
	}
	sb.WriteString(`</ul>
</body>
</html>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, sb.String())
}

// downloadHandler 处理文件或文件夹下载请求
// 使用 GET 方法，查询参数 "path" 指定路径
// 如果是文件夹，会打包成 ZIP 下载
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Missing path parameter", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(uploadDir, filepath.Base(path)) // 安全路径

	// 检查路径是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		http.Error(w, "Path not found", http.StatusNotFound)
		return
	}

	// 检查是否为目录
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		// 打包目录为 ZIP
		zipName := path + ".zip"
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", zipName))

		// 创建 ZIP 并写入响应
		zipWriter := zip.NewWriter(w)
		defer zipWriter.Close()

		err := zipDir(zipWriter, fullPath, "")
		if err != nil {
			http.Error(w, "Failed to zip directory", http.StatusInternalServerError)
			return
		}
	} else {
		// 单个文件下载
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(path)))
		http.ServeFile(w, r, fullPath)
	}
}

// extractZip 解压 ZIP 文件到指定目录
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	log.Printf("Starting extraction to %s", destDir)

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		// 检查路径安全
		if !strings.HasPrefix(fpath, filepath.Clean(destDir)) {
			log.Printf("Illegal path detected: %s", fpath)
			return fmt.Errorf("illegal file path")
		}

		if f.FileInfo().IsDir() {
			log.Printf("Creating directory: %s", fpath)
			if err := os.MkdirAll(fpath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			log.Printf("Error creating parent dir for %s: %v", fpath, err)
			return err
		}

		log.Printf("Extracting file: %s to %s", f.Name, fpath)

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			log.Printf("Error opening output file %s: %v", fpath, err)
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			log.Printf("Error opening ZIP entry %s: %v", f.Name, err)
			return err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			log.Printf("Error copying %s: %v", f.Name, err)
			return err
		}

		log.Printf("Successfully extracted: %s", fpath)
	}

	log.Printf("Extraction completed for %s", destDir)
	return nil
}

// zipDir 将目录打包到 ZIP 写入器
func zipDir(zw *zip.Writer, root string, base string) error {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if base != "" {
			relPath = filepath.Join(base, relPath)
		}

		if info.IsDir() {
			_, err = zw.Create(relPath + "/")
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		w, err := zw.Create(relPath)
		if err != nil {
			return err
		}

		buf := make([]byte, 32*1024) // 32KB 缓冲
		_, err = io.CopyBuffer(w, f, buf)
		return err
	})
	return err
}
