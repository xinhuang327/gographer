# Gographer
Write GraphQL schema manually in original graphql-js or graphql-go is really boring, there is obviously so much duplicated information.With Gographer,
you can generate GraphQL schema on the fly with much clear code. This work is based on Graphql-go project.


With this tool, you can define GraphQL schema with something like below, which is much compact.
```golang
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


**Still working in progress, have a look at the source, PR is much welcomed.**