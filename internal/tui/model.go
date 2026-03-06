package tui

type Model struct {
	quitting bool
}

func NewModel() Model {
	return Model{}
}

