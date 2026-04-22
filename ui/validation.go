package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/AhmedAburady/banana-cli/api"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

// GenerateFormData holds validated data for generate form
type GenerateFormData struct {
	OutputFolder  string
	NumImages     int
	Prompt        string
	AspectRatio   string
	ImageSize     string
	Model         string
	ThinkingLevel string
	Grounding     bool
	ImageSearch   bool
}

// EditFormData holds validated data for edit form
type EditFormData struct {
	ReferencePath string
	OutputFolder  string
	NumImages     int
	Prompt        string
	AspectRatio   string
	ImageSize     string
	Model         string
	ThinkingLevel string
	Grounding     bool
	ImageSearch   bool
}

// ValidateRequired checks if a value is not empty
func ValidateRequired(field, value string) error {
	if value == "" {
		return ValidationError{Field: field, Message: fmt.Sprintf("%s is required", field)}
	}
	return nil
}

// ValidateNumImages validates the number of images (1-20)
func ValidateNumImages(value string) (int, error) {
	if value == "" {
		return 5, nil // default
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > 20 {
		return 0, ValidationError{Field: "num", Message: "Number of images must be 1-20"}
	}
	return n, nil
}

// ValidateOutputFolder validates the output folder path
func ValidateOutputFolder(path string) (string, error) {
	if path == "" {
		return ".", nil // default to current directory
	}

	// Expand tilde to home directory
	path = api.ExpandTilde(path)

	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return "", ValidationError{Field: "output", Message: "Output path is not a directory"}
		}
		return path, nil
	}

	if os.IsNotExist(err) {
		// Check parent directory exists
		parent := filepath.Dir(path)
		if _, err := os.Stat(parent); os.IsNotExist(err) {
			return "", ValidationError{Field: "output", Message: "Output parent directory does not exist"}
		}
		return path, nil
	}

	// Handle other errors (permission denied, etc.)
	return "", ValidationError{Field: "output", Message: fmt.Sprintf("Cannot access path: %v", err)}
}

// ValidateReferencePath validates the reference image/folder path
func ValidateReferencePath(path string) error {
	if path == "" {
		return ValidationError{Field: "ref", Message: "Reference path is required"}
	}

	// Expand tilde to home directory
	path = api.ExpandTilde(path)

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return ValidationError{Field: "ref", Message: "Path does not exist"}
	}
	if err != nil {
		return ValidationError{Field: "ref", Message: "Cannot access path"}
	}

	if info.IsDir() {
		count, _ := api.FindImagesInDir(path)
		if count == 0 {
			return ValidationError{Field: "ref", Message: "No images found in directory"}
		}
	} else {
		if !api.IsSupportedImage(path) {
			return ValidationError{Field: "ref", Message: "Unsupported image format"}
		}
	}

	return nil
}

// ValidateGenerateForm validates all fields for generate form
func ValidateGenerateForm(form *Form) (GenerateFormData, error) {
	// Prompt is required
	prompt := form.GetString("prompt")
	if err := ValidateRequired("Prompt", prompt); err != nil {
		return GenerateFormData{}, err
	}

	// Number of images
	numImages, err := ValidateNumImages(form.GetString("num"))
	if err != nil {
		return GenerateFormData{}, err
	}

	// Output folder
	outputFolder, err := ValidateOutputFolder(form.GetString("output"))
	if err != nil {
		return GenerateFormData{}, err
	}

	return GenerateFormData{
		OutputFolder:  outputFolder,
		NumImages:     numImages,
		Prompt:        prompt,
		AspectRatio:   form.GetString("aspect"),
		ImageSize:     form.GetString("size"),
		Model:         form.GetString("model"),
		ThinkingLevel: form.GetString("thinking"),
		Grounding:     form.GetBool("grounding"),
		ImageSearch:   form.GetBool("imagesearch"),
	}, nil
}

// ValidateEditForm validates all fields for edit form
func ValidateEditForm(form *Form, defaultPrompt string) (EditFormData, error) {
	// Reference path is required
	refPath := form.GetString("ref")
	if err := ValidateReferencePath(refPath); err != nil {
		return EditFormData{}, err
	}

	// Number of images
	numImages, err := ValidateNumImages(form.GetString("num"))
	if err != nil {
		return EditFormData{}, err
	}

	// Output folder
	outputFolder, err := ValidateOutputFolder(form.GetString("output"))
	if err != nil {
		return EditFormData{}, err
	}

	// Prompt (use default if empty)
	prompt := form.GetString("prompt")
	if prompt == "" {
		prompt = defaultPrompt
	}

	return EditFormData{
		ReferencePath: refPath,
		OutputFolder:  outputFolder,
		NumImages:     numImages,
		Prompt:        prompt,
		AspectRatio:   form.GetString("aspect"),
		ImageSize:     form.GetString("size"),
		Model:         form.GetString("model"),
		ThinkingLevel: form.GetString("thinking"),
		Grounding:     form.GetBool("grounding"),
		ImageSearch:   form.GetBool("imagesearch"),
	}, nil
}
