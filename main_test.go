package main

import (
	"testing"
)

const failureFormat = "Expected %v to equal %v"

// Event Interface Tests

// TodoListCreated

func TestTodoListCreatedGetID(t *testing.T) {
	event := TodoListCreated{"id", "name"}
	actual, expected := event.getID(), "id"
	if actual != expected {
		t.Errorf(failureFormat, actual, expected)
	}
}

func TestTodoListCreatedGetName(t *testing.T) {
	event := TodoListCreated{"id", "name"}
	actual, expected := event.getName(), "name"
	if actual != expected {
		t.Errorf(failureFormat, actual, expected)
	}
}
