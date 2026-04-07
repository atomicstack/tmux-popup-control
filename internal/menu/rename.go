package menu

type RenamePrompt struct {
	Context Context
	Target  string
	Initial string
}

type RenameRequest struct {
	Context Context
	Target  string
	Value   string
}

type PanePrompt = RenamePrompt
type WindowPrompt = RenamePrompt
