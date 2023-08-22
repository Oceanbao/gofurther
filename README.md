# Go Prod

## Project Structure

- `bin` contains compiled app binaries, ready for deployment to prod server.
- `cmd/api` app-specific code for API, including server runner, read/write HTTP requests and auth
- `internal` various ancillary packages used by API, database, validation, email, etc. In short non-app and reusable utilities (internal is being imported only)
- `migrations` SQL migration files for database
- `remote` config files and setup scripts for server

## PG

- Local dev requires `brew install golang-migration`
- To setup migration, need to run as superuser inside `pq` terminal:

```sql
GRANT ALL ON DATABASE greenlight TO greenlight;
ALTER DATABASE greenlight OWNER TO greenlight;
```

- To create a migration:

```bash
migrate create -seq -ext=.sql -dir=./migrations create_users_table
```

## Database

- Many-to-many relationship is often modelled via a joining table
- For example, `user` and `permissions` will have a joining table `users_permissions` that stores `user_id` and `permission_id` in which each user may have multiple permissions
- Query can come from both side, so again the clean way is to create handlers for both sides of the use case, e.g. `PermissionModel.GetAllForUser(user) []Permission` and `UserModel.GetAllForPermission(permission) []User`

## Security

- If API endpoint requires credentials (cookies or HTTP basic authentication) need to set `Access-Control-Allow-Credentials: true` header in response. To send credentials with cross-origin request then need to specify this in `fetch(URL, { credenials: 'include' })`
- What is 'simple cross-origin request': satisfying all of - HTTP is one of the 3 CORS-safe methods: HEAD, GET, POST; request header either forbidden or one of four CORS-safe (`Accept, Accept-Language, Content-Language, Content-Type`); `Content-Type` (if set) is one of (`application/x-www-form-urlencoded, multipart/form-data, text/plain`)
- If not above, browser will trigger an initial "preflight" request BEFORE the real one to scout the real one will be permitted or not.
- To handler "preflight" request, rely on 3 flags: `OPTIONS` method, `Origin` header and `Access-Control-Request-Method` header, always present. Once confirmed, respond with 200 OK and special headers to let browser know whether or not it's OK for the real request to proceed: `Access-Control-Allow-Origin`, `Access-Control-Allow-Methods` listing HTTP methods allowed for the real cross-origin requests to the URL and `Access-Control-Allow-Headers` listing headers that can be included in real one. (e.g. ...Origin: `<reflected trusted origin>`, ...Methods: OPTIONS, PUT, PATCH, DELETE, ...Headers: Authorization, Content-Type)
- KEY: `Authorization` header allowed in cross-origin requests must not be paired with `*` Origins, or insecure.
- `Access-Control-Max-Age` is appealing but also risky (unable to clear cache if mistake in setting) (-1 is disable) (browser set their hard max)

## Swagger API

- `@Param [param_name][param_type] [data_type][required/mandatory] [description]`
- `[param_type] can be query, path, header, body, formData`

```go
// Comment Format
// success	Success response that separated by spaces. return code or default,{param type},data type,comment
// failure	Failure response that separated by spaces. return code or default,{param type},data type,comment
// response	As same as success and failure
// header	Header in response that separated by spaces. return code,{param type},data type,comment

//	@Param	enumstring	query	string		false	"string enums"		Enums(A, B, C)
//	@Param	enumint		query	int			false	"int enums"			Enums(1, 2, 3)
//	@Param	enumnumber	query	number		false	"int enums"			Enums(1.1, 1.2, 1.3)
//	@Param	string		query	string		false	"string valid"		minlength(5)	maxlength(10)
//	@Param	int			query	int			false	"int valid"			minimum(1)		maximum(10)
//	@Param	default		query	string		false	"string default"	default(A)
//	@Param	example		query	string		false	"string example"	example(string)
//	@Param	collection	query	[]string	false	"string collection"	collectionFormat(multi)
//	@Param	extensions	query	[]string	false	"string collection"	extensions(x-example=test,x-nullable)
```

## Missing Pieces

- More complex SQL features: statement and transaction and other details `
- More HTML templating.
- Middleware helper lib: alice
- Forms
- Stateful HTTP: session management
- TLS and CSRF
- More embed
- Testing
