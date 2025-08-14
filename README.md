# PDF Merger & Image Converter

A Go web application that allows users to merge PDF files and automatically convert PNG/JPG images to PDF format before merging.

## Features

- ✅ Upload multiple files (PDF, PNG, JPG/JPEG)
- ✅ Automatic image to PDF conversion
- ✅ Merge all files into a single PDF
- ✅ Download the merged PDF
- ✅ Drag and drop file upload
- ✅ Clean, responsive web interface
- ✅ File size display and management

## Requirements

- Go 1.21 or later
- Internet connection for downloading dependencies

## Installation & Setup

1. **Clone or navigate to the project directory:**
   ```bash
   cd /Users/kalana/dev/me/pdfmg
   ```

2. **Initialize Go modules and download dependencies:**
   ```bash
   go mod tidy
   ```

3. **Run the application:**
   ```bash
   go run main.go
   ```

4. **Open your browser and navigate to:**
   ```
   http://localhost:8080
   ```

## How to Use

1. **Upload Files:**
   - Click on the upload area or drag and drop files
   - Select multiple PDF, PNG, or JPG files
   - Supported formats: `.pdf`, `.png`, `.jpg`, `.jpeg`

2. **Review Selected Files:**
   - View the list of selected files with their sizes
   - Remove any unwanted files using the "Remove" button

3. **Merge Files:**
   - Click the "Merge Files" button
   - Wait for the processing to complete

4. **Download Result:**
   - Click the "Download" button to save the merged PDF
   - The file will be saved to your default download location

## Project Structure

```
pdfmg/
├── main.go           # Main application code
├── go.mod           # Go module definition
├── go.sum           # Go module checksums (generated)
├── uploads/         # Temporary storage for uploaded files (auto-created)
├── output/          # Storage for merged PDF files (auto-created)
└── README.md        # This file
```

## Dependencies

- **github.com/pdfcpu/pdfcpu** - PDF processing and merging
- **github.com/jung-kurt/gofpdf** - PDF generation for image conversion
- **github.com/disintegration/imaging** - Image processing and manipulation

## API Endpoints

- `GET /` - Main web interface
- `POST /upload` - File upload and processing endpoint
- `GET /download/{filename}` - Download merged PDF files

## Configuration

The application runs on port 8080 by default. You can change this by setting the `PORT` environment variable:

```bash
PORT=3000 go run main.go
```

## File Processing

1. **Image to PDF Conversion:**
   - Images are automatically resized to fit A4 pages
   - Maintains aspect ratio
   - Centers images on the page

2. **PDF Merging:**
   - Uses pdfcpu library for reliable PDF merging
   - Maintains original PDF quality
   - Handles various PDF versions and formats

## Troubleshooting

**Issue: "Module not found" errors**
```bash
go mod tidy
go mod download
```

**Issue: Port already in use**
```bash
# Use a different port
PORT=8081 go run main.go
```

**Issue: Files not uploading**
- Check file formats (only PDF, PNG, JPG supported)
- Ensure files are under 32MB each
- Check browser console for JavaScript errors

## Development

To modify the application:

1. **Frontend changes:** Edit the HTML template in the `handleIndex` function
2. **Backend logic:** Modify the respective handler functions
3. **Add new file formats:** Extend the `convertToPDF` function

## Security Notes

- Files are temporarily stored in the `uploads` directory
- Merged PDFs are stored in the `output` directory
- Temporary files are cleaned up after processing
- No persistent storage of user files

## License

This project is open source. Feel free to modify and distribute as needed.
