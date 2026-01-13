package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aiserve/gpuproxy/internal/models"
)

var (
	modelPath    string
	modelName    string
	modelFormat  string
	gpuType      string
	framework    string
	endpoint     string
	replicas     int
	autoDetect   bool
	listFormats  bool
)

func main() {
	flag.StringVar(&modelPath, "model", "", "Path to model file or directory (required)")
	flag.StringVar(&modelName, "name", "", "Model name (default: filename)")
	flag.StringVar(&modelFormat, "format", "", "Model format (pickle, onnx, tensorflow, keras, pytorch, pmml, joblib, tensorrt, coreml, tflite)")
	flag.StringVar(&gpuType, "gpu", "", "Preferred GPU type (H100, A100, V100, etc.)")
	flag.StringVar(&framework, "framework", "", "Framework (pytorch, tensorflow, sklearn, etc.)")
	flag.StringVar(&endpoint, "endpoint", "", "Custom endpoint path (default: /serve/models/{id}/predict)")
	flag.IntVar(&replicas, "replicas", 1, "Number of model replicas for load balancing")
	flag.BoolVar(&autoDetect, "auto", false, "Auto-detect model format from file extension")
	flag.BoolVar(&listFormats, "list-formats", false, "List supported model formats and exit")
	flag.Parse()

	if listFormats {
		printSupportedFormats()
		os.Exit(0)
	}

	if modelPath == "" {
		fmt.Println("Error: --model is required")
		flag.Usage()
		os.Exit(1)
	}

	// Check if model file exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		log.Fatalf("Model file not found: %s", modelPath)
	}

	// Auto-detect format if requested or not specified
	if autoDetect || modelFormat == "" {
		detected := models.DetectModelFormat(modelPath)
		if detected == "" {
			log.Fatalf("Could not auto-detect model format. Please specify with --format")
		}
		modelFormat = string(detected)
		log.Printf("Auto-detected format: %s", modelFormat)
	}

	// Use filename as name if not specified
	if modelName == "" {
		modelName = filepath.Base(modelPath)
	}

	// Print serving configuration
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("GPU Proxy Model Serving")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Model:      %s\n", modelName)
	fmt.Printf("Path:       %s\n", modelPath)
	fmt.Printf("Format:     %s\n", modelFormat)
	if framework != "" {
		fmt.Printf("Framework:  %s\n", framework)
	}
	if gpuType != "" {
		fmt.Printf("GPU Type:   %s\n", gpuType)
	}
	fmt.Printf("Replicas:   %d\n", replicas)
	fmt.Println(strings.Repeat("=", 60))

	// Create model configuration
	config := map[string]interface{}{
		"model_path": modelPath,
		"name":       modelName,
		"format":     modelFormat,
		"framework":  framework,
		"gpu_type":   gpuType,
		"replicas":   replicas,
	}

	if endpoint != "" {
		config["endpoint"] = endpoint
	}

	// Instructions for uploading model
	fmt.Println("\nTo serve this model:")
	fmt.Println("\n1. Upload via API:")
	fmt.Printf("   curl -X POST http://localhost:8080/api/v1/models/upload \\\n")
	fmt.Printf("     -H \"X-API-Key: YOUR_API_KEY\" \\\n")
	fmt.Printf("     -F \"model=@%s\" \\\n", modelPath)
	fmt.Printf("     -F \"name=%s\" \\\n", modelName)
	fmt.Printf("     -F \"format=%s\" \\\n", modelFormat)
	if framework != "" {
		fmt.Printf("     -F \"framework=%s\" \\\n", framework)
	}
	if gpuType != "" {
		fmt.Printf("     -F \"gpu_type=%s\" \\\n", gpuType)
		fmt.Printf("     -F \"gpu_required=true\" \\\n")
	}
	fmt.Printf("     -F \"replicas=%d\"\n", replicas)

	fmt.Println("\n2. Or use the client:")
	fmt.Printf("   ./aiserve-gpuproxy-client model upload \\\n")
	fmt.Printf("     --file %s \\\n", modelPath)
	fmt.Printf("     --name %s \\\n", modelName)
	fmt.Printf("     --format %s\n", modelFormat)

	fmt.Println("\n3. Test prediction:")
	fmt.Println("   curl -X POST http://localhost:8080/api/v1/models/{MODEL_ID}/predict \\")
	fmt.Println("     -H \"X-API-Key: YOUR_API_KEY\" \\")
	fmt.Println("     -H \"Content-Type: application/json\" \\")
	fmt.Println("     -d '{\"inputs\": {\"data\": [[1,2,3,4]]}}'")

	fmt.Println("\n4. Monitor metrics:")
	fmt.Println("   curl http://localhost:8080/api/v1/models/{MODEL_ID}/metrics")

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Model configuration ready for deployment!")
	fmt.Println(strings.Repeat("=", 60))
}

func printSupportedFormats() {
	fmt.Println("Supported Model Formats:")
	fmt.Println(strings.Repeat("=", 80))

	formats := []struct {
		format      string
		extensions  string
		framework   string
		description string
	}{
		{
			"pickle",
			".pkl, .pickle",
			"scikit-learn, PyTorch",
			"Python pickle format for ML models",
		},
		{
			"onnx",
			".onnx",
			"ONNX (cross-framework)",
			"Open Neural Network Exchange format",
		},
		{
			"tensorflow",
			".pb, saved_model.pb",
			"TensorFlow",
			"TensorFlow SavedModel format",
		},
		{
			"keras",
			".h5, .hdf5",
			"Keras/TensorFlow",
			"Keras HDF5 model format",
		},
		{
			"pytorch",
			".pt, .pth",
			"PyTorch",
			"PyTorch model checkpoint",
		},
		{
			"pmml",
			".pmml",
			"Multiple",
			"Predictive Model Markup Language",
		},
		{
			"joblib",
			".joblib",
			"scikit-learn",
			"JobLib serialization format",
		},
		{
			"tensorrt",
			".plan, .trt",
			"NVIDIA TensorRT",
			"Optimized for NVIDIA GPUs",
		},
		{
			"coreml",
			".mlmodel",
			"Apple Core ML",
			"Apple Core ML format",
		},
		{
			"tflite",
			".tflite",
			"TensorFlow Lite",
			"TensorFlow Lite for edge devices",
		},
	}

	fmt.Printf("%-15s %-25s %-25s %s\n", "FORMAT", "EXTENSIONS", "FRAMEWORK", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 80))

	for _, f := range formats {
		fmt.Printf("%-15s %-25s %-25s %s\n", f.format, f.extensions, f.framework, f.description)
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("\nUsage Examples:")
	fmt.Println("  # Auto-detect format and serve:")
	fmt.Println("  ./serve-model --model model.onnx --auto")
	fmt.Println()
	fmt.Println("  # Specify format explicitly:")
	fmt.Println("  ./serve-model --model model.pkl --format pickle --framework sklearn")
	fmt.Println()
	fmt.Println("  # Serve with specific GPU:")
	fmt.Println("  ./serve-model --model model.pt --format pytorch --gpu H100")
	fmt.Println()
	fmt.Println("  # Multiple replicas for high availability:")
	fmt.Println("  ./serve-model --model model.onnx --replicas 3")
}
