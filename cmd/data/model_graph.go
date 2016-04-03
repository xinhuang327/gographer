package data

import (
	"github.com/graphql-go/relay"
)

type Root struct{}

type Mutation struct{}

type AddTodoInput struct {
	Text string `nonnull:"true"`
}

func (m *Mutation) AddTodo(in AddTodoInput) (todoEdge relay.EdgeType, viewer *User) {
	todoId := AddTodo(in.Text, false)
	todo := GetTodo(todoId)
	// TODO: manage pagination
	return relay.EdgeType{
		Node:   todo,
		Cursor: relay.CursorForObjectInConnection(TodosToSliceInterface(GetTodos("any")), todo),
	}, GetViewer()
}

func (m *Mutation) ChangeTodoStatus(id string, complete bool) (todo *Todo, viewer *User) {
	resolvedId := relay.FromGlobalID(id) // TODO: ID conversion could be handled outside the function
	todoID := resolvedId.ID
	ChangeTodoStatus(todoID, complete)
	return GetTodo(todoID), GetViewer()
}

func (m *Mutation) MarkAllTodos(complete bool) (changedTodos []*Todo, viewer *User) {
	todoIds := MarkAllTodos(complete)
	todos := []*Todo{}
	for _, todoId := range todoIds {
		todo := GetTodo(todoId)
		if todo != nil {
			todos = append(todos, todo)
		}
	}
	return todos, GetViewer()
}

func (m *Mutation) RemoveCompletedTodos() (deletedTodoIds []string, viewer *User) {
	return RemoveCompletedTodos(), GetViewer()
}

func (m *Mutation) RemoveTodo(id string) (deletedTodoId string, viewer *User) {
	resolvedId := relay.FromGlobalID(id)
	RemoveTodo(resolvedId.ID)
	return relay.ToGlobalID(resolvedId.Type, resolvedId.ID), GetViewer()
}

func (m *Mutation) RenameTodo(id string, text string) (todo *Todo, viewer *User) {
	resolvedId := relay.FromGlobalID(id)
	todoID := resolvedId.ID
	RenameTodo(todoID, text)
	return GetTodo(todoID), GetViewer()
}

func (r *Root) GetViewer() *User {
	return GetViewer()
}

// Struct arg's field name must be exported (Upper case first letter, will use lower case first letter in GraphQL)
type GetTodosInput struct {
	Status string `def:"any"`
}

func (u *User) GetTodos(p GetTodosInput) []*Todo {
	return GetTodos(p.Status)
}

func (u *User) GetTotalCount() int {
	return len(GetTodos("any"))
}

func (u *User) GetCompletedCount() int {
	return len(GetTodos("completed"))
}