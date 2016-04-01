# Gographer
Write GraphQL schema manually in original graphql-js or graphql-go is really boring, there is obviously so much duplicated information.With Gographer,
you can generate GraphQL schema on the fly with much clear code. This work is based on Graphql-go project.


With this tool, you can define GraphQL schema with something like below, which is much compact.
```golang
func GetModelSchemaInfo() *gographer.SchemaInfo {
	sch := gographer.NewSchemaInfo()

	sch.RegType(Todo{}).
	SetIDResolver(func(id string) interface{} {
		return GetTodo(id)
	}).
	IDField("id", nil).
	SimpleField("text").
	SimpleField("complete")

	sch.RegType(User{}).
	SetIDResolver(func(id string) interface{} {
		return GetUser(id)
	}).
	IDField("id", nil).
	ResolvedField("todos", "GetTodos", "status", "any").
	ResolvedField("totalCount", "GetTotalCount").
	ResolvedField("completedCount", "GetCompletedCount")

	sch.RegType(&Root{}).SetRoot().
	ResolvedField("viewer", "GetViewer")

	return sch
}
```


**Still working in progress, have a look at the source, PR is much welcomed.**