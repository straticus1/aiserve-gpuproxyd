package ml

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PyTorchConverter handles conversion of PyTorch models to ONNX format
// This allows users to upload .pt/.pth files and serve them via ONNX Runtime
type PyTorchConverter struct {
	pythonPath      string
	conversionScript string
	tempDir         string
}

// ConversionResult contains details about a conversion
type ConversionResult struct {
	Success       bool      `json:"success"`
	ONNXPath      string    `json:"onnx_path"`
	OriginalPath  string    `json:"original_path"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	ConversionTime float64  `json:"conversion_time_seconds"`
	InputShapes   [][]int64 `json:"input_shapes"`
	OutputShapes  [][]int64 `json:"output_shapes"`
}

// NewPyTorchConverter creates a new PyTorch to ONNX converter
func NewPyTorchConverter(pythonPath string) *PyTorchConverter {
	if pythonPath == "" {
		pythonPath = "python3" // Use system Python by default
	}

	return &PyTorchConverter{
		pythonPath:      pythonPath,
		conversionScript: "/tmp/pytorch_to_onnx.py",
		tempDir:         "/tmp/model_conversions",
	}
}

// Initialize sets up the converter (creates temp directories, writes scripts)
func (c *PyTorchConverter) Initialize() error {
	// Create temp directory
	if err := os.MkdirAll(c.tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Write Python conversion script
	script := c.generateConversionScript()
	if err := os.WriteFile(c.conversionScript, []byte(script), 0644); err != nil {
		return fmt.Errorf("failed to write conversion script: %w", err)
	}

	return nil
}

// ConvertToONNX converts a PyTorch model (.pt/.pth) to ONNX format
func (c *PyTorchConverter) ConvertToONNX(ctx context.Context, modelPath string, inputShapes [][]int64, outputPath string) (*ConversionResult, error) {
	start := time.Now()

	result := &ConversionResult{
		OriginalPath: modelPath,
		ONNXPath:     outputPath,
	}

	// Validate input file exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("model file not found: %s", modelPath)
		return result, fmt.Errorf("model file not found: %s", modelPath)
	}

	// Generate output path if not provided
	if outputPath == "" {
		ext := filepath.Ext(modelPath)
		outputPath = strings.TrimSuffix(modelPath, ext) + ".onnx"
	}

	// Prepare input shapes argument
	inputShapesJSON, err := json.Marshal(inputShapes)
	if err != nil {
		result.Success = false
		result.ErrorMessage = "failed to serialize input shapes"
		return result, fmt.Errorf("failed to serialize input shapes: %w", err)
	}

	// Execute Python conversion script
	cmd := exec.CommandContext(ctx, c.pythonPath, c.conversionScript,
		"--model-path", modelPath,
		"--output-path", outputPath,
		"--input-shapes", string(inputShapesJSON),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Success = false
		result.ErrorMessage = string(output)
		return result, fmt.Errorf("conversion failed: %w\nOutput: %s", err, output)
	}

	// Parse conversion result
	var scriptResult struct {
		Success      bool      `json:"success"`
		InputShapes  [][]int64 `json:"input_shapes"`
		OutputShapes [][]int64 `json:"output_shapes"`
		Error        string    `json:"error,omitempty"`
	}

	if err := json.Unmarshal(output, &scriptResult); err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("failed to parse conversion output: %v", err)
		return result, fmt.Errorf("failed to parse conversion output: %w", err)
	}

	result.Success = scriptResult.Success
	result.InputShapes = scriptResult.InputShapes
	result.OutputShapes = scriptResult.OutputShapes
	result.ErrorMessage = scriptResult.Error
	result.ConversionTime = time.Since(start).Seconds()

	if !result.Success {
		return result, fmt.Errorf("conversion failed: %s", result.ErrorMessage)
	}

	return result, nil
}

// AutoConvert automatically converts PyTorch models to ONNX with sensible defaults
func (c *PyTorchConverter) AutoConvert(ctx context.Context, modelPath string) (*ConversionResult, error) {
	// Common input shapes for different model types
	defaultShapes := [][]int64{
		{1, 3, 224, 224}, // Image classification (ResNet, VGG, etc.)
	}

	// Try conversion with default shapes
	outputPath := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".onnx"
	return c.ConvertToONNX(ctx, modelPath, defaultShapes, outputPath)
}

// generateConversionScript creates the Python script for PyTorch → ONNX conversion
func (c *PyTorchConverter) generateConversionScript() string {
	return `#!/usr/bin/env python3
"""
PyTorch to ONNX Converter
Converts PyTorch models (.pt, .pth) to ONNX format for inference
"""
import argparse
import json
import sys
import torch
import torch.onnx

def convert_pytorch_to_onnx(model_path, output_path, input_shapes):
    """Convert PyTorch model to ONNX format"""
    try:
        # Load PyTorch model
        model = torch.load(model_path, map_location='cpu')

        # Handle different PyTorch save formats
        if isinstance(model, dict):
            # State dict format - need to reconstruct model
            # This is a limitation - user must provide model architecture
            return {
                "success": False,
                "error": "Model is saved as state_dict. Please save full model with torch.save(model, path) or provide model architecture."
            }

        model.eval()

        # Prepare dummy inputs based on input_shapes
        dummy_inputs = []
        for shape in input_shapes:
            dummy_inputs.append(torch.randn(*shape))

        if len(dummy_inputs) == 1:
            dummy_inputs = dummy_inputs[0]
        else:
            dummy_inputs = tuple(dummy_inputs)

        # Export to ONNX
        torch.onnx.export(
            model,
            dummy_inputs,
            output_path,
            export_params=True,
            opset_version=17,  # ONNX opset version
            do_constant_folding=True,
            input_names=['input'],
            output_names=['output'],
            dynamic_axes={
                'input': {0: 'batch_size'},
                'output': {0: 'batch_size'}
            }
        )

        # Get output shapes by running dummy inference
        with torch.no_grad():
            output = model(dummy_inputs)
            if isinstance(output, tuple):
                output_shapes = [list(o.shape) for o in output]
            else:
                output_shapes = [list(output.shape)]

        return {
            "success": True,
            "input_shapes": input_shapes,
            "output_shapes": output_shapes
        }

    except Exception as e:
        return {
            "success": False,
            "error": str(e)
        }

def main():
    parser = argparse.ArgumentParser(description='Convert PyTorch model to ONNX')
    parser.add_argument('--model-path', required=True, help='Path to PyTorch model (.pt or .pth)')
    parser.add_argument('--output-path', required=True, help='Path for output ONNX model')
    parser.add_argument('--input-shapes', required=True, help='JSON array of input shapes')

    args = parser.parse_args()

    # Parse input shapes
    input_shapes = json.loads(args.input_shapes)

    # Convert model
    result = convert_pytorch_to_onnx(args.model_path, args.output_path, input_shapes)

    # Output result as JSON
    print(json.dumps(result))

    # Exit with appropriate code
    sys.exit(0 if result['success'] else 1)

if __name__ == '__main__':
    main()
`
}

// ValidateONNX checks if an ONNX model is valid
func (c *PyTorchConverter) ValidateONNX(ctx context.Context, onnxPath string) error {
	// Basic validation - check file exists and has .onnx extension
	if _, err := os.Stat(onnxPath); os.IsNotExist(err) {
		return fmt.Errorf("ONNX file not found: %s", onnxPath)
	}

	if filepath.Ext(onnxPath) != ".onnx" {
		return fmt.Errorf("file does not have .onnx extension: %s", onnxPath)
	}

	// TODO: Could add more sophisticated validation using ONNX checker
	// This would require Python onnx package: onnx.checker.check_model()

	return nil
}

// CleanupTempFiles removes temporary conversion files
func (c *PyTorchConverter) CleanupTempFiles() error {
	if err := os.RemoveAll(c.tempDir); err != nil {
		return fmt.Errorf("failed to cleanup temp directory: %w", err)
	}
	return nil
}

// GetSupportedFormats returns the PyTorch formats this converter supports
func (c *PyTorchConverter) GetSupportedFormats() []string {
	return []string{
		".pt",   // PyTorch saved model (full model)
		".pth",  // PyTorch saved model (full model or state dict)
	}
}

// InstallationInstructions returns instructions for installing PyTorch
func (c *PyTorchConverter) InstallationInstructions() string {
	return `
To enable PyTorch to ONNX conversion, install PyTorch:

CPU Only:
  pip install torch torchvision torchaudio

GPU (CUDA 11.8):
  pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu118

GPU (CUDA 12.1):
  pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu121

Verify installation:
  python3 -c "import torch; print(torch.__version__)"
`
}
