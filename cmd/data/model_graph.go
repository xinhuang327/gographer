package data

import (
	"github.com/graphql-go/relay"
)

type Root struct{}

type Mutation struct{}

type AddTodoInput struct {
	Text string `nonNull:"true"`
}

type AddTodoOutput struct {
	TodoEdge relay.EdgeType //`elemType:"Todo"`
	Viewer   *User
}

func (m *Mutation) AddTodo(in AddTodoInput) *AddTodoOutput {
	todoId := AddTodo(in.Text, false)
	todo := GetTodo(todoId)
	// TODO: manage pagination
	return &AddTodoOutput{
		TodoEdge: relay.EdgeType{
			Node:   todo,
			Cursor: relay.CursorForObjectInConnection(TodosToSliceInterface(GetTodos("any")), todo),
		},
		Viewer: GetViewer(),
	}
}

type ChangeTodoStatusInput struct {
	Id       string `nonNull:"true"`
	Complete bool   `nonNull:"true"`
}

type ChangeTodoStatusOutput struct {
	Todo   *Todo
	Viewer *User
}

func (m *Mutation) ChangeTodoStatus(in ChangeTodoStatusInput) *ChangeTodoStatusOutput {
	resolvedId := relay.FromGlobalID(in.Id) // TODO: ID conversion could be handled outside the function
	todoID := resolvedId.ID
	ChangeTodoStatus(todoID, in.Complete)
	return &ChangeTodoStatusOutput{GetTodo(todoID), GetViewer()}
}

type MarkAllTodosInput struct {
	Complete bool `nonNull:"true"`
}

type MarkAllTodosOutput struct {
	ChangedTodosConnection []*Todo `json:"changedTodos"`
	Viewer                 *User
}

func (m *Mutation) MarkAllTodos(in MarkAllTodosInput) *MarkAllTodosOutput {
	todoIds := MarkAllTodos(in.Complete)
	todos := []*Todo{}
	for _, todoId := range todoIds {
		todo := GetTodo(todoId)
		if todo != nil {
			todos = append(todos, todo)
		}
	}
	return &MarkAllTodosOutput{todos, GetViewer()}
}

type RemoveCompletedTodosOutput struct {
	DeletedTodoIds []string
	Viewer         *User
}

func (m *Mutation) RemoveCompletedTodos() *RemoveCompletedTodosOutput {
	return &RemoveCompletedTodosOutput{RemoveCompletedTodos(), GetViewer()}
}

type RemoveTodoInput struct {
	Id string `nonNull:"true"`
}

type RemoveTodoOutput struct {
	DeletedTodoId string
	Viewer        *User
}

func (m *Mutation) RemoveTodo(in RemoveTodoInput) *RemoveTodoOutput {
	resolvedId := relay.FromGlobalID(in.Id)
	RemoveTodo(resolvedId.ID)
	return &RemoveTodoOutput{relay.ToGlobalID(resolvedId.Type, resolvedId.ID), GetViewer()}
}

type RenameTodoInput struct {
	Id   string `nonNull:"true"`
	Text string `nonNull:"true"`
}

func (m *Mutation) RenameTodo(in RenameTodoInput) *ChangeTodoStatusOutput {
	resolvedId := relay.FromGlobalID(in.Id)
	todoID := resolvedId.ID
	RenameTodo(todoID, in.Text)
	return &ChangeTodoStatusOutput{GetTodo(todoID), GetViewer()}
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
