package ui

// FormConfig defines a form configuration
type FormConfig struct {
	Title  string
	Fields []FieldConfig
}

// FieldConfig defines a field configuration
type FieldConfig struct {
	Type           FieldType
	Key            string
	Label          string
	Description    string
	Placeholder    string
	Default        string
	BoolDefault    bool
	Lines          int            // For textarea
	DirsOnly       bool           // For path
	AllowedExts    []string       // For path
	Options        []SelectOption // For select
	DefaultIdx     int            // For select
	InlineWithPrev bool           // Render side-by-side with previous field
}

// Common field configs shared between forms
var (
	OutputFolderField = FieldConfig{
		Type:        FieldPath,
		Key:         "output",
		Label:       "Output Folder",
		Placeholder: "./output",
		Default:     ".",
		DirsOnly:    true,
	}

	NumImagesField = FieldConfig{
		Type:        FieldInput,
		Key:         "num",
		Label:       "Number of Images",
		Placeholder: "1-20",
		Default:     "5",
	}

	AspectRatioField = FieldConfig{
		Type:       FieldSelect,
		Key:        "aspect",
		Label:      "Aspect Ratio",
		Options:    AspectRatioOptions(),
		DefaultIdx: 0,
	}

	ImageSizeField = FieldConfig{
		Type:           FieldSelect,
		Key:            "size",
		Label:          "Image Size",
		Options:        ImageSizeOptions(),
		DefaultIdx:     1, // 2K default
		InlineWithPrev: true,
	}

	ModelField = FieldConfig{
		Type:       FieldSelect,
		Key:        "model",
		Label:      "Model",
		Options:    ModelOptions(),
		DefaultIdx: 0, // Pro
	}

	ThinkingLevelField = FieldConfig{
		Type:           FieldSelect,
		Key:            "thinking",
		Label:          "Thinking Level",
		Options:        ThinkingLevelOptions(),
		DefaultIdx:     0, // Minimal
		InlineWithPrev: true,
	}

	GroundingField = FieldConfig{
		Type:        FieldToggle,
		Key:         "grounding",
		Label:       "Google Search",
		BoolDefault: false,
	}

	ImageSearchField = FieldConfig{
		Type:           FieldToggle,
		Key:            "imagesearch",
		Label:          "Image Search",
		BoolDefault:    false,
		InlineWithPrev: true,
	}
)

// GenerateFormConfig returns the configuration for the generate form
func GenerateFormConfig() FormConfig {
	return FormConfig{
		Title: "Generate Image",
		Fields: []FieldConfig{
			OutputFolderField,
			{
				Type:        FieldTextArea,
				Key:         "prompt",
				Label:       "Prompt",
				Placeholder: "A beautiful sunset over mountains...",
				Lines:       3,
			},
			NumImagesField,
			ImageSizeField,
			AspectRatioField,
			ModelField,
			ThinkingLevelField,
			GroundingField,
			ImageSearchField,
		},
	}
}

// EditFormConfig returns the configuration for the edit form
func EditFormConfig() FormConfig {
	imageExts := []string{".png", ".jpg", ".jpeg", ".gif", ".webp"}

	return FormConfig{
		Title: "Edit Image",
		Fields: []FieldConfig{
			{
				Type:        FieldPath,
				Key:         "ref",
				Label:       "Reference Input",
				Placeholder: "./refs or ./image.png",
				Default:     "",
				DirsOnly:    false,
				AllowedExts: imageExts,
			},
			OutputFolderField,
			{
				Type:        FieldTextArea,
				Key:         "prompt",
				Label:       "Prompt",
				Placeholder: "A 2D vector art pattern inspired by the reference...",
				Lines:       3,
			},
			NumImagesField,
			ImageSizeField,
			AspectRatioField,
			ModelField,
			ThinkingLevelField,
			GroundingField,
			ImageSearchField,
		},
	}
}

// BuildForm creates a Form from a FormConfig
func BuildForm(config FormConfig) *Form {
	form := NewForm(config.Title)

	for _, field := range config.Fields {
		switch field.Type {
		case FieldInput:
			form.AddInput(field.Key, field.Label, field.Description, field.Placeholder, field.Default)
		case FieldTextArea:
			form.AddTextArea(field.Key, field.Label, field.Description, field.Placeholder, field.Lines)
		case FieldSelect:
			form.AddSelect(field.Key, field.Label, field.Description, field.Options, field.DefaultIdx)
		case FieldToggle:
			form.AddToggle(field.Key, field.Label, field.Description, field.BoolDefault)
		case FieldPath:
			form.AddPath(field.Key, field.Label, field.Description, field.Placeholder, field.Default, field.DirsOnly, field.AllowedExts)
		}
		// Propagate InlineWithPrev to the FormField
		if field.InlineWithPrev {
			form.Fields[len(form.Fields)-1].InlineWithPrev = true
		}
	}

	return form
}
