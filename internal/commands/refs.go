package commands

import (
	"werk/internal/models"
)

func resolveID(id string) (string, error) {
	return database.ResolveTaskID(id)
}

func resolveRef(id string) (string, error) {
	t, err := database.GetTask(id)
	if err != nil {
		return "", err
	}
	return t.Ref, nil
}

func mustResolveID(id string) string {
	resolvedID, err := resolveID(id)
	if err != nil {
		outputError(err.Error())
	}
	return resolvedID
}

func mustResolveRef(id string) string {
	ref, err := resolveRef(id)
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

func toTaskJSONSlice(tasks []models.Task) []models.TaskJSON {
	out := make([]models.TaskJSON, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.ToJSON())
	}
	return out
}
