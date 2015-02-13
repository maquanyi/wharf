package routers

import (
	"github.com/astaxie/beego"
	"github.com/dockercn/wharf/controllers"
)

func init() {
	//Web Interface
	beego.Router("/", &controllers.MainController{})
	beego.Router("/auth", &controllers.AuthController{}, "get:Get")
	//beego.Router("/signout", &controllers.AuthWebController{}, "get:Signout")
	beego.Router("/setting", &controllers.DashboardController{}, "get:GetSetting")
	beego.Router("/dashboard", &controllers.DashboardController{}, "get:GetDashboard")
	//	beego.Router("/admin", &controllers.AdminController{}, "get:GetAdmin")
	//beego.Router("/repositories/add", &controllers.RepositoryController{}, "get:GetRepositoryAdd")

	// //Static File
	// beego.Router("/favicon.ico", &controllers.StaticController{}, "get:GetFavicon")
	// //TODO sitemap/rss/robots.txt

	web := beego.NewNamespace("/w1",
		beego.NSRouter("/signin", &controllers.AuthWebController{}, "post:Signin"),
		//		beego.NSRouter("/reset", &controllers.AuthWebController{}, "post:ResetPasswd"),
		beego.NSRouter("/signup", &controllers.AuthWebController{}, "post:Signup"),
		beego.NSRouter("/profile", &controllers.UsersWebController{}, "get:GetProfile"),

		//team routers
		beego.NSRouter("/users/:username", &controllers.UsersWebController{}, "get:GetUserExist"),
		beego.NSRouter("/team", &controllers.TeamWebController{}, "post:PostTeam"),

		//organization routers
		beego.NSRouter("/organizations", &controllers.OrganizationWebController{}, "get:GetOrganizations"),
		beego.NSRouter("/organization", &controllers.OrganizationWebController{}, "post:PostOrganization"),
		beego.NSRouter("/organization", &controllers.OrganizationWebController{}, "put:PutOrganization"),
		beego.NSRouter("/organizations/:orgName", &controllers.OrganizationWebController{}, "get:GetOrganizationDetail"),

		//		beego.NSRouter("/profile", &controllers.UsersWebController{}, "put:PutProfile"),
		//		beego.NSRouter("/account", &controllers.UsersWebController{}, "put:PutAccount"),
		//		beego.NSRouter("/gravatar", &controllers.UsersWebController{}, "post:PostGravatar"),
	)

	// //CI Service API
	// drone := beego.NewNamespace("/d1",
	//	beego.NSRouter("/yaml", &controllers.DroneAPIController{}, "post:PostYAML"),
	// )

	//Docker Registry API V1 remain
	beego.Router("/_ping", &controllers.PingAPIController{}, "get:GetPing")
	// beego.Router("/_status", &controllers.StatusAPIController{})

	// //Docker Registry API V1 目前不支持V2 协议
	api := beego.NewNamespace("/v1",
		beego.NSRouter("/_ping", &controllers.PingAPIController{}, "get:GetPing"),
		//	beego.NSRouter("/_status", &controllers.StatusAPIController{}),
		beego.NSRouter("/users", &controllers.UsersAPIController{}, "get:GetUsers"),
		beego.NSRouter("/users", &controllers.UsersAPIController{}, "post:PostUsers"),

		beego.NSNamespace("/repositories",
			beego.NSRouter("/:namespace/:repo_name/tags/:tag", &controllers.RepositoryAPIController{}, "put:PutTag"),
			beego.NSRouter("/:namespace/:repo_name/images", &controllers.RepositoryAPIController{}, "put:PutRepositoryImages"),
			beego.NSRouter("/:namespace/:repo_name/images", &controllers.RepositoryAPIController{}, "get:GetRepositoryImages"),
			beego.NSRouter("/:namespace/:repo_name/tags", &controllers.RepositoryAPIController{}, "get:GetRepositoryTags"),
			beego.NSRouter("/:namespace/:repo_name", &controllers.RepositoryAPIController{}, "put:PutRepository"),
		),

		beego.NSNamespace("/images",
			beego.NSRouter("/:image_id/ancestry", &controllers.ImageAPIController{}, "get:GetImageAncestry"),
			beego.NSRouter("/:image_id/json", &controllers.ImageAPIController{}, "get:GetImageJSON"),
			beego.NSRouter("/:image_id/layer", &controllers.ImageAPIController{}, "get:GetImageLayer"),
			beego.NSRouter("/:image_id/json", &controllers.ImageAPIController{}, "put:PutImageJSON"),
			beego.NSRouter("/:image_id/layer", &controllers.ImageAPIController{}, "put:PutImageLayer"),
			beego.NSRouter("/:image_id/checksum", &controllers.ImageAPIController{}, "put:PutChecksum"),
		),
	)

	beego.AddNamespace(web)
	//beego.AddNamespace(drone)
	beego.AddNamespace(api)
}
