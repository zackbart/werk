package commands

import (
	"werk/internal/models"
)

func resolveID(idOrRef string) (string, error) {
	return database.ResolveTaskID(idOrRef)
}

func resolveRef(idOrRef string) (string, error) {
	return database.ResolveTaskRef(idOrRef)
}

func mustResolveID(idOrRef string) string {
	id, err := resolveID(idOrRef)
	if err != nil {
		outputError(err.Error())
	}
	return id
}

func mustResolveRef(idOrRef string) string {
	ref, err := resolveRef(idOrRef)
	if err != nil {
		outputError(err.Error())
	}
	return ref
}

func toTaskJSONList(tasks []models.Task) []interface{} {
	out := make([]interface{}, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.ToJSON())
	}
	return out
}
