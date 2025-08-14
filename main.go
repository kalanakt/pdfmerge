package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/jung-kurt/gofpdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type FileHandler struct {
	uploadsDir string
	outputDir  string
}

func NewFileHandler() *FileHandler {
	uploadsDir := "uploads"
	outputDir := "output"

	// Create directories if they don't exist
	os.MkdirAll(uploadsDir, 0755)
	os.MkdirAll(outputDir, 0755)

	return &FileHandler{
		uploadsDir: uploadsDir,
		outputDir:  outputDir,
	}
}

func (fh *FileHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(32 << 20) // 32MB max
	if err != nil {
		http.Error(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	var convertedPDFs []string
	timestamp := time.Now().Format("20060102_150405")

	// Process each uploaded file
	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Error opening file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Save uploaded file
		fileName := fmt.Sprintf("%s_%d_%s", timestamp, i, fileHeader.Filename)
		uploadPath := filepath.Join(fh.uploadsDir, fileName)

		dst, err := os.Create(uploadPath)
		if err != nil {
			http.Error(w, "Error creating file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		_, err = io.Copy(dst, file)
		if err != nil {
			http.Error(w, "Error saving file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Convert to PDF if necessary
		pdfPath, err := fh.convertToPDF(uploadPath, fileHeader.Filename)
		if err != nil {
			http.Error(w, "Error converting file to PDF: "+err.Error(), http.StatusInternalServerError)
			return
		}

		convertedPDFs = append(convertedPDFs, pdfPath)
	}

	// Merge all PDFs
	mergedPath, err := fh.mergePDFs(convertedPDFs, timestamp)
	if err != nil {
		http.Error(w, "Error merging PDFs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Clean up temporary files
	for _, path := range convertedPDFs {
		if !strings.Contains(path, fh.outputDir) {
			os.Remove(path)
		}
	}

	// Return success response with download link
	response := map[string]string{
		"status":      "success",
		"downloadUrl": "/download/" + filepath.Base(mergedPath),
		"filename":    filepath.Base(mergedPath),
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "%s", "downloadUrl": "%s", "filename": "%s"}`,
		response["status"], response["downloadUrl"], response["filename"])
}

func (fh *FileHandler) convertToPDF(filePath, originalName string) (string, error) {
	ext := strings.ToLower(filepath.Ext(originalName))

	// If already PDF, return as is
	if ext == ".pdf" {
		return filePath, nil
	}

	// Convert image to PDF
	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
		return fh.imageToPDF(filePath, originalName)
	}

	return "", fmt.Errorf("unsupported file format: %s", ext)
}

func (fh *FileHandler) imageToPDF(imagePath, originalName string) (string, error) {
	// Open and decode image
	img, err := imaging.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("error opening image: %v", err)
	}

	// Create PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Get image dimensions
	bounds := img.Bounds()
	imgWidth := float64(bounds.Dx())
	imgHeight := float64(bounds.Dy())

	// Calculate scaling to fit A4 page (210x297mm with margins)
	pageWidth := 190.0  // A4 width minus margins
	pageHeight := 277.0 // A4 height minus margins

	scale := 1.0
	if imgWidth > pageWidth || imgHeight > pageHeight {
		scaleX := pageWidth / imgWidth
		scaleY := pageHeight / imgHeight
		scale = scaleX
		if scaleY < scaleX {
			scale = scaleY
		}
	}

	finalWidth := imgWidth * scale
	finalHeight := imgHeight * scale

	// Center the image on page
	x := (210 - finalWidth) / 2
	y := (297 - finalHeight) / 2

	// Convert image to temporary file for gofpdf
	tempImagePath := strings.TrimSuffix(imagePath, filepath.Ext(imagePath)) + "_temp.png"
	err = imaging.Save(img, tempImagePath)
	if err != nil {
		return "", fmt.Errorf("error saving temporary image: %v", err)
	}
	defer os.Remove(tempImagePath)

	// Add image to PDF
	pdf.Image(tempImagePath, x, y, finalWidth, finalHeight, false, "", 0, "")

	// Save PDF
	pdfPath := strings.TrimSuffix(imagePath, filepath.Ext(imagePath)) + ".pdf"
	err = pdf.OutputFileAndClose(pdfPath)
	if err != nil {
		return "", fmt.Errorf("error creating PDF: %v", err)
	}

	// Clean up original image file
	os.Remove(imagePath)

	return pdfPath, nil
}

func (fh *FileHandler) mergePDFs(pdfPaths []string, timestamp string) (string, error) {
	if len(pdfPaths) == 0 {
		return "", fmt.Errorf("no PDF files to merge")
	}

	if len(pdfPaths) == 1 {
		// If only one PDF, move it to output directory
		outputPath := filepath.Join(fh.outputDir, fmt.Sprintf("merged_%s.pdf", timestamp))
		err := copyFile(pdfPaths[0], outputPath)
		return outputPath, err
	}

	// Merge multiple PDFs
	outputPath := filepath.Join(fh.outputDir, fmt.Sprintf("merged_%s.pdf", timestamp))

	// Use pdfcpu to merge PDFs
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed

	err := api.MergeCreateFile(pdfPaths, outputPath, false, conf)
	if err != nil {
		return "", fmt.Errorf("error merging PDFs: %v", err)
	}

	return outputPath, nil
}

func (fh *FileHandler) handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	if filename == "" {
		http.Error(w, "No filename specified", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(fh.outputDir, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Set headers for PDF download
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Serve the file
	http.ServeFile(w, r, filePath)
}

func (fh *FileHandler) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PDF Merger</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            background-color: white;
            padding: 30px;
            border-radius: 10px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 {
            color: #333;
            text-align: center;
            margin-bottom: 30px;
        }
        .upload-area {
            border: 2px dashed #ccc;
            border-radius: 10px;
            padding: 40px;
            text-align: center;
            margin-bottom: 20px;
            transition: border-color 0.3s;
        }
        .upload-area:hover {
            border-color: #007bff;
        }
        .upload-area.dragover {
            border-color: #007bff;
            background-color: #f8f9ff;
        }
        #fileInput {
            display: none;
        }
        .file-label {
            cursor: pointer;
            color: #007bff;
            font-size: 18px;
        }
        .file-list {
            margin: 20px 0;
        }
        .file-item {
            background-color: #f8f9fa;
            padding: 10px;
            margin: 5px 0;
            border-radius: 5px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            cursor: move;
            transition: background-color 0.2s;
        }
        .file-item:hover {
            background-color: #e9ecef;
        }
        .file-item.dragging {
            opacity: 0.5;
            background-color: #dee2e6;
        }
        .file-item.drag-over {
            border-top: 3px solid #007bff;
        }
        .drag-handle {
            color: #6c757d;
            margin-right: 10px;
            cursor: move;
        }
        .file-item .remove-btn {
            background-color: #dc3545;
            color: white;
            border: none;
            padding: 5px 10px;
            border-radius: 3px;
            cursor: pointer;
        }
        .merge-btn {
            background-color: #28a745;
            color: white;
            border: none;
            padding: 15px 30px;
            border-radius: 5px;
            cursor: pointer;
            font-size: 16px;
            width: 100%;
            margin-top: 20px;
        }
        .merge-btn:disabled {
            background-color: #ccc;
            cursor: not-allowed;
        }
        .result {
            margin-top: 20px;
            padding: 15px;
            border-radius: 5px;
        }
        .success {
            background-color: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }
        .error {
            background-color: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }
        .download-btn {
            background-color: #007bff;
            color: white;
            border: none;
            padding: 10px 20px;
            border-radius: 5px;
            cursor: pointer;
            text-decoration: none;
            display: inline-block;
            margin-top: 10px;
        }
        .loading {
            display: none;
            text-align: center;
            margin: 20px 0;
        }
        .spinner {
            border: 4px solid #f3f3f3;
            border-top: 4px solid #3498db;
            border-radius: 50%;
            width: 40px;
            height: 40px;
            animation: spin 2s linear infinite;
            margin: 0 auto;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>PDF Merger & Image Converter</h1>
        <p style="text-align: center; color: #666;">
            Select multiple PDF, PNG, or JPG files to merge into a single PDF
        </p>
        
        <div class="upload-area" id="uploadArea">
            <label for="fileInput" class="file-label">
                📁 Click here to select files or drag and drop them
            </label>
            <input type="file" id="fileInput" multiple accept=".pdf,.png,.jpg,.jpeg">
        </div>
        
        <div class="file-list" id="fileList"></div>
        
        <button class="merge-btn" id="mergeBtn" disabled onclick="mergePDFs()">
            Merge Files
        </button>
        
        <div class="loading" id="loading">
            <div class="spinner"></div>
            <p>Processing files...</p>
        </div>
        
        <div id="result"></div>
    </div>

    <script>
        let selectedFiles = [];
        const fileInput = document.getElementById('fileInput');
        const fileList = document.getElementById('fileList');
        const mergeBtn = document.getElementById('mergeBtn');
        const uploadArea = document.getElementById('uploadArea');
        const loading = document.getElementById('loading');
        const result = document.getElementById('result');

        // Handle file selection
        fileInput.addEventListener('change', function(e) {
            handleFiles(e.target.files);
        });

        // Handle drag and drop
        uploadArea.addEventListener('dragover', function(e) {
            e.preventDefault();
            uploadArea.classList.add('dragover');
        });

        uploadArea.addEventListener('dragleave', function(e) {
            e.preventDefault();
            uploadArea.classList.remove('dragover');
        });

        uploadArea.addEventListener('drop', function(e) {
            e.preventDefault();
            uploadArea.classList.remove('dragover');
            handleFiles(e.dataTransfer.files);
        });

        function handleFiles(files) {
            for (let file of files) {
                if (file.type === 'application/pdf' || 
                    file.type.startsWith('image/png') || 
                    file.type.startsWith('image/jpeg') ||
                    file.name.toLowerCase().endsWith('.pdf') ||
                    file.name.toLowerCase().endsWith('.png') ||
                    file.name.toLowerCase().endsWith('.jpg') ||
                    file.name.toLowerCase().endsWith('.jpeg')) {
                    selectedFiles.push(file);
                }
            }
            updateFileList();
        }

        function updateFileList() {
            fileList.innerHTML = '';
            selectedFiles.forEach((file, index) => {
                const fileItem = document.createElement('div');
                fileItem.className = 'file-item';
                fileItem.draggable = true;
                fileItem.dataset.index = index;
                fileItem.innerHTML = ` + "`" + `
                    <div style="display: flex; align-items: center;">
                        <span class="drag-handle">⋮⋮</span>
                        <span>${file.name} (${(file.size / 1024 / 1024).toFixed(2)} MB)</span>
                    </div>
                    <button class="remove-btn" onclick="removeFile(${index})">Remove</button>
                ` + "`" + `;
                
                // Add drag event listeners
                fileItem.addEventListener('dragstart', handleDragStart);
                fileItem.addEventListener('dragover', handleDragOver);
                fileItem.addEventListener('drop', handleDrop);
                fileItem.addEventListener('dragend', handleDragEnd);
                fileItem.addEventListener('dragenter', handleDragEnter);
                fileItem.addEventListener('dragleave', handleDragLeave);
                
                fileList.appendChild(fileItem);
            });
            
            mergeBtn.disabled = selectedFiles.length === 0;
        }

        function removeFile(index) {
            selectedFiles.splice(index, 1);
            updateFileList();
        }

        // Drag and drop reordering functionality
        let draggedIndex = null;

        function handleDragStart(e) {
            draggedIndex = parseInt(e.target.dataset.index);
            e.target.classList.add('dragging');
            e.dataTransfer.effectAllowed = 'move';
        }

        function handleDragEnd(e) {
            e.target.classList.remove('dragging');
            draggedIndex = null;
            
            // Remove all drag-over classes
            document.querySelectorAll('.file-item').forEach(item => {
                item.classList.remove('drag-over');
            });
        }

        function handleDragOver(e) {
            e.preventDefault();
            e.dataTransfer.dropEffect = 'move';
        }

        function handleDragEnter(e) {
            e.preventDefault();
            if (e.target.classList.contains('file-item') && draggedIndex !== null) {
                const targetIndex = parseInt(e.target.dataset.index);
                if (targetIndex !== draggedIndex) {
                    e.target.classList.add('drag-over');
                }
            }
        }

        function handleDragLeave(e) {
            if (e.target.classList.contains('file-item')) {
                e.target.classList.remove('drag-over');
            }
        }

        function handleDrop(e) {
            e.preventDefault();
            
            if (draggedIndex === null) return;
            
            const targetIndex = parseInt(e.target.dataset.index);
            
            if (targetIndex !== draggedIndex) {
                // Reorder the files array
                const draggedFile = selectedFiles[draggedIndex];
                selectedFiles.splice(draggedIndex, 1);
                selectedFiles.splice(targetIndex, 0, draggedFile);
                
                // Update the display
                updateFileList();
            }
            
            // Clean up
            e.target.classList.remove('drag-over');
        }

        async function mergePDFs() {
            if (selectedFiles.length === 0) return;

            loading.style.display = 'block';
            result.innerHTML = '';
            mergeBtn.disabled = true;

            const formData = new FormData();
            selectedFiles.forEach(file => {
                formData.append('files', file);
            });

            try {
                const response = await fetch('/upload', {
                    method: 'POST',
                    body: formData
                });

                const data = await response.json();

                if (response.ok && data.status === 'success') {
                    result.innerHTML = ` + "`" + `
                        <div class="result success">
                            <strong>Success!</strong> Your PDF has been merged successfully.
                            <br>
                            <a href="${data.downloadUrl}" class="download-btn" download>
                                📥 Download ${data.filename}
                            </a>
                        </div>
                    ` + "`" + `;
                } else {
                    throw new Error(data.error || 'Unknown error occurred');
                }
            } catch (error) {
                result.innerHTML = ` + "`" + `
                    <div class="result error">
                        <strong>Error:</strong> ${error.message}
                    </div>
                ` + "`" + `;
            } finally {
                loading.style.display = 'none';
                mergeBtn.disabled = false;
            }
        }
    </script>
</body>
</html>
	`

	t, err := template.New("index").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	t.Execute(w, nil)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func main() {
	fh := NewFileHandler()

	http.HandleFunc("/", fh.handleIndex)
	http.HandleFunc("/upload", fh.handleUpload)
	http.HandleFunc("/download/", fh.handleDownload)

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	log.Printf("Server starting on port %s", port)
	log.Printf("Open http://localhost:%s in your browser", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
