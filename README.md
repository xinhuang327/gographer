# Gographer
**Still working in progress, have a look at the source if you are interested, I don't think there is a library alike in Go right now, PR is much welcomed.**

Write GraphQL schema manually really tedious, it also mix model and logic code together, there is obviously much duplicated information.

With Gographer,
you can generate GraphQL schema on the fly with much clear code. This work is based on Graphql-go project's GraphQL implementation.

The idea is somewhat like python's graphene, but more flexible, it won't force you to rewrite any code, you can adapt your existing model and logic code to GraphQL schema pretty easily.
The implementation uses Golang's reflection package heavily, it sure will not be performant as hand written static code, but can reduce more than 60% lines of code, and they are much clear than the raw style.

Currently I'm trying to add enough features to utilize it in my project, before majority of the design been finished, and API been stablized, there won't be much test coverage.

Automatically bind/write Golang model to GraphQL:

* Struct field
* Struct methods to computed field/resolved field
* Mutation type and function
* Argument and return value
* Embedded struct field
* Extension field addon for existing code


With this tool, you can define GraphQL schema with something like below, which is much compact. You can see the full example in cmd/data folder, in which are schema definition to match original GraphQL TodoMVC example.
```go
func GetModelSchemaInfo() *gg.SchemaInfo {
	sch := gg.NewSchemaInfo()

	sch.RegType(Todo{}).
		SetIDResolver(func(id string) interface{} {
			return GetTodo(id)
		}).
		IDField("id", nil).SimpleFields()

	sch.RegType(User{}).
		SetIDResolver(func(id string) interface{} {
			return GetUser(id)
		}).
		IDField("id", nil).ResolvedFields()

	sch.RegType(&Root{}).SetRoot().ResolvedFields()

	sch.RegType(&Mutation{}).SetMutation().MutationFields()

	return sch
}


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
```


