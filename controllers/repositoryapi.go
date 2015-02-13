/*
Docker Push & Pull

执行 docker push 命令流程：
    1. docker 向 registry 服务器注册 repository： PUT /v1/repositories/<username>/<repository> -> PUTRepository()
    2. 参数是 JSON 格式的 <repository> 所有 image 的 id 列表，按照 image 的构建顺序排列。
    3. 根据 <repository> 的 <tags> 进行循环：
       3.1 获取 <image> 的 JSON 文件：GET /v1/images/<image_id>/json -> image.go#GETJSON()
       3.2 如果没有此文件或内容返回 404 。
       3.3 docker push 认为服务器没有 image 对应的文件，向服务器上传 image 相关文件。
           3.3.1 写入 <image> 的 JSON 文件：PUT /v1/images/<image_id>/json -> image.go#PUTJSON()
           3.3.2 写入 <image> 的 layer 文件：PUT /v1/images/<image_id>/layer -> image.go#PUTLayer()
           3.3.3 写入 <image> 的 checksum 信息：PUT /v1/images/<image_id>/checksum -> image.go#PUTChecksum()
       3.4 上传完此 tag 的所有 image 后，向服务器写入 tag 信息：PUT /v1/repositories/(namespace)/(repository)/tags/(tag) -> PUTTag()
    4. 所有 tags 的 image 上传完成后，向服务器发送所有 images 的校验信息，PUT /v1/repositories/(namespace)/(repo_name)/images -> PUTRepositoryImages()

执行 docker pull 命令流程：
    1. docker 访问 registry 服务器 repository 的 images 信息：GET /v1/repositories/<username>/<repository>/images -> GetRepositoryImages()
    2. docker 访问 registry 服务器 repository 的 tags 信息：GET /v1/repositoies/<username>/<repository>/tags -> GetRepositoryTags()
    3. 根据 <repository> 的 <tags> 中 image 信息进行循环：
      3.1 获取 <image> 的 Ancestry 信息：GET /v1/images/<image_id>/ancestry -> GetImageAncestry()
      3.2 获取 <image> 的 JSON 数据: GET /v1/images/<image_id>/json -> GetImageJson()
      3.3 获取 <image> 的 Layer 文件: GET /v1/images/<image_id/layer -> GetImageLayer()

*/
package controllers

import (
	"fmt"
	"net/http"
	"regexp"
	//	"strings"
	"github.com/astaxie/beego"
	"github.com/dockercn/wharf/models"
	"github.com/dockercn/wharf/utils"
)

type RepositoryAPIController struct {
	beego.Controller
}

func (r *RepositoryAPIController) URLMapping() {
	r.Mapping("PutTag", r.PutTag)
	r.Mapping("PutRepositoryImages", r.PutRepositoryImages)
	r.Mapping("GetRepositoryImages", r.GetRepositoryImages)
	r.Mapping("GetRepositoryTags", r.GetRepositoryTags)
	r.Mapping("PutRepository", r.PutRepository)
}

func (this *RepositoryAPIController) Prepare() {
	beego.Debug(fmt.Sprintf("[%s] %s | %s", this.Ctx.Input.Host(), this.Ctx.Input.Request.Method, this.Ctx.Input.Request.RequestURI))

	beego.Debug("[Headers]")
	beego.Debug(this.Ctx.Input.Request.Header)

	//相应 docker api 命令的 Controller 屏蔽 beego 的 XSRF ，避免错误。
	this.EnableXSRF = false

	//设置 Response 的 Header 信息，在处理函数中可以覆盖
	this.Ctx.Output.Context.ResponseWriter.Header().Set("Content-Type", "application/json;charset=UTF-8")
	this.Ctx.Output.Context.ResponseWriter.Header().Set("X-Docker-Registry-Standalone", beego.AppConfig.String("docker::Standalone"))
	this.Ctx.Output.Context.ResponseWriter.Header().Set("X-Docker-Registry-Version", beego.AppConfig.String("docker::Version"))
	this.Ctx.Output.Context.ResponseWriter.Header().Set("X-Docker-Registry-Config", beego.AppConfig.String("docker::Config"))
	this.Ctx.Output.Context.ResponseWriter.Header().Set("X-Docker-Encrypt", beego.AppConfig.String("docker::Encrypt"))

}

func (this *RepositoryAPIController) PutRepository() {
	isAuth, errCode, errInfo := models.DoAuthPutRepository(this.Ctx)
	if !isAuth {
		this.Ctx.Output.Context.Output.SetStatus(errCode)
		this.Ctx.Output.Context.Output.Body(errInfo)
		this.StopRun()
	}
	username, passwd, _ := utils.DecodeBasicAuth(this.Ctx.Input.Header("Authorization"))
	//获取namespace/repository
	namespace := string(this.Ctx.Input.Param(":namespace"))
	repository := string(this.Ctx.Input.Param(":repo_name"))

	//加密签名
	sign := ""
	if len(string(this.Ctx.Input.Header("X-Docker-Sign"))) > 0 {
		sign = string(this.Ctx.Input.Header("X-Docker-Sign"))
	}

	beego.Debug("[Sign] " + sign)

	//创建或更新 Repository 数据
	//也可以采用 ioutil.ReadAll(this.Ctx.Request.Body) 的方式读取 body 数据
	//TODO 检查 JSON 字符串是否合法
	//TODO 检查 逻辑是否合法

	beego.Debug("[JSON] " + string(this.Ctx.Input.CopyBody()))

	//从 API 创建的 Repository 默认是 Public 的。
	repo := new(models.Repository)
	if err := repo.DoPut(namespace, repository, string(this.Ctx.Input.CopyBody()), this.Ctx.Input.Header("User-Agent")); err != nil {
		beego.Error(fmt.Sprintf("[API 用户] Put repository 错误: %s", err.Error()))
		this.Ctx.Output.Context.Output.SetStatus(http.StatusForbidden)
		this.Ctx.Output.Context.Output.Body([]byte(`{"错误":"Put repository 错误"}`))
		this.StopRun()
	}

	//如果 Request 的 Header 中含有 X-Docker-Token 且为 True，需要在返回值设置 Token 值。
	//否则客户端报错 Index response didn't contain an access token
	if this.Ctx.Input.Header("X-Docker-Token") == "true" {
		//创建 token 并保存
		//需要加密的字符串为 UserName + UserPassword + 时间戳
		token := utils.GeneralToken(username + passwd)
		this.SetSession("token", token)
		//在返回值 Header 里面设置 Token
		this.Ctx.Output.Context.ResponseWriter.Header().Set("X-Docker-Token", token)
		this.Ctx.Output.Context.ResponseWriter.Header().Set("WWW-Authenticate", token)
	}

	this.SetSession("username", username)
	//	this.SetSession("org", org)
	this.SetSession("namespace", namespace)
	this.SetSession("repository", repository)
	this.SetSession("access", "write")

	//操作正常的输出
	this.Ctx.Output.Context.ResponseWriter.Header().Set("X-Docker-Endpoints", beego.AppConfig.String("docker::Endpoints"))
	this.Ctx.Output.Context.Output.SetStatus(http.StatusOK)
	this.Ctx.Output.Context.Output.Body([]byte("\"\""))

	return

}

func (this *RepositoryAPIController) PutTag() {

	isAuth, errCode, errInfo := models.DoAuthPutRepositoryTag(this.Ctx)

	if !isAuth {
		this.Ctx.Output.Context.Output.SetStatus(errCode)
		this.Ctx.Output.Context.Output.Body(errInfo)
		this.StopRun()
	}

	beego.Debug("[Namespace] " + this.Ctx.Input.Param(":namespace"))
	beego.Debug("[Repository] " + this.Ctx.Input.Param(":repo_name"))
	beego.Debug("[Tag] " + this.Ctx.Input.Param(":tag"))
	beego.Debug("[Session username] " + this.GetSession("username").(string))
	//	beego.Debug("[Session org] " + this.GetSession("org").(string))

	//username := this.GetSession("username").(string)
	//org := this.GetSession("org").(string)

	namespace := this.Ctx.Input.Param(":namespace")
	repository := this.Ctx.Input.Param(":repo_name")

	//加密签名
	sign := ""
	if len(string(this.Ctx.Input.Header("X-Docker-Sign"))) > 0 {
		sign = string(this.Ctx.Input.Header("X-Docker-Sign"))
	}

	beego.Debug("[Sign] " + sign)

	tag := this.Ctx.Input.Param(":tag")

	//从 HTTP Body 中获取 Image 的 Value
	r, _ := regexp.Compile(`"([[:alnum:]]+)"`)
	imageIds := r.FindStringSubmatch(string(this.Ctx.Input.CopyBody()))

	repo := new(models.Repository)
	if err := repo.PutTag(imageIds[1], namespace, repository, tag); err != nil {
		beego.Error(fmt.Sprintf("[API 用户] 更新 %s/%s 的 Tag [%s:%s] 错误: %s", namespace, repository, imageIds[1], tag, err.Error()))
		this.Ctx.Output.Context.Output.SetStatus(http.StatusBadRequest)
		this.Ctx.Output.Context.Output.Body([]byte("{\"错误\":\"更新 Tag 数据错误\"}"))
		this.StopRun()
	}

	//操作正常的输出
	this.Ctx.Output.Context.Output.SetStatus(http.StatusOK)
	this.Ctx.Output.Context.Output.Body([]byte("\"\""))
}

//Push 命令的最后一步，所有的检查操作，通知操作都在此函数进行。
func (this *RepositoryAPIController) PutRepositoryImages() {

	isAuth, errCode, errInfo := models.DoAuthPutRepositoryImage(this.Ctx)

	if !isAuth {
		this.Ctx.Output.Context.Output.SetStatus(errCode)
		this.Ctx.Output.Context.Output.Body(errInfo)
		this.StopRun()
	}

	beego.Debug("[Namespace] " + this.Ctx.Input.Param(":namespace"))
	beego.Debug("[Repository] " + this.Ctx.Input.Param(":repo_name"))
	//beego.Debug("[Session username] " + this.GetSession("username").(string))
	//beego.Debug("[Session org] " + this.GetSession("org").(string))

	//	username := this.GetSession("username").(string)
	//	org := this.GetSession("org").(string)

	namespace := this.Ctx.Input.Param(":namespace")
	repository := this.Ctx.Input.Param(":repo_name")

	//加密签名
	sign := ""
	if len(string(this.Ctx.Input.Header("X-Docker-Sign"))) > 0 {
		sign = string(this.Ctx.Input.Header("X-Docker-Sign"))
	}

	beego.Debug("[Sign] " + sign)
	beego.Debug("[Body] " + string(this.Ctx.Input.CopyBody()))

	repo := new(models.Repository)

	//TODO 检查仓库所有镜像的 Tag 信息和上传的 Tag 信息是否一致。
	//TODO 检查仓库所有镜像是否 Uploaded 为 True
	//TODO 检查仓库所有镜像是否 Checksumed 为 True

	//设定 repository 的 Uploaded
	if err := repo.PutImages(namespace, repository); err != nil {
		beego.Error(fmt.Sprintf("[API 用户] 更新 %s/%s 的 Uploaded 标志错误: %s", namespace, repository, err.Error()))
		this.Ctx.Output.Context.Output.SetStatus(http.StatusBadRequest)
		this.Ctx.Output.Context.Output.Body([]byte("{\"错误\":\"更新 Uploaded 标志错误\"}"))
		this.StopRun()
	}

	//操作正常的输出
	this.Ctx.Output.Context.Output.SetStatus(http.StatusNoContent)
	this.Ctx.Output.Context.Output.Body([]byte("\"\""))
}

//获取一个 Repository 的 Image 信息
func (this *RepositoryAPIController) GetRepositoryImages() {

	isAuth, errCode, errInfo := models.DoAuthGetRepositoryImages(this.Ctx)

	if !isAuth {
		this.Ctx.Output.Context.Output.SetStatus(errCode)
		this.Ctx.Output.Context.Output.Body(errInfo)
		this.StopRun()
	}
	username, passwd, _ := utils.DecodeBasicAuth(this.Ctx.Input.Header("Authorization"))
	//获取namespace/repository
	namespace := string(this.Ctx.Input.Param(":namespace"))
	repository := string(this.Ctx.Input.Param(":repo_name"))

	//	orgModel := new(models.Organization)
	//	has, _, _ := orgModel.Has(namespace)
	//	org := ""
	//	if has == true {
	//		org = namespace
	//	}

	//	beego.Debug("[Username] " + username)
	//	beego.Debug("[Org] " + org)
	beego.Debug("[Repository] " + repository)
	beego.Debug("[namespace] " + namespace)

	//TODO：私有和组织的镜像仓库权限判断问题

	//加密签名
	sign := ""
	if len(string(this.Ctx.Input.Header("X-Docker-Sign"))) > 0 {
		sign = string(this.Ctx.Input.Header("X-Docker-Sign"))
	}

	beego.Debug("[Sign] " + sign)

	//TODO 私有镜像仓库权限判断

	repo := new(models.Repository)

	isHas, _, err := repo.Has(namespace, repository)
	if err != nil {
		beego.Error(fmt.Sprintf("[API 用户] 读取 %s/%s 的 JSON 数据错误: %s", namespace, repository, err.Error()))
		this.Ctx.Output.Context.Output.SetStatus(http.StatusBadRequest)
		this.Ctx.Output.Context.Output.Body([]byte("{\"错误\":\"读取 JSON 数据错误\"}"))
		this.StopRun()

	}
	if !isHas {

		beego.Error(fmt.Sprintf("[API 用户] 没有找到 %s/%s", namespace, repository))
		this.Ctx.Output.Context.Output.SetStatus(http.StatusBadRequest)
		this.Ctx.Output.Context.Output.Body([]byte(fmt.Sprintf("没有找到 %s/%s", namespace, repository)))
		this.StopRun()

	}

	//如果 Request 的 Header 中含有 X-Docker-Token 且为 True，需要在返回值设置 Token 值。
	//否则客户端报错 Index response didn't contain an access token
	if this.Ctx.Input.Header("X-Docker-Token") == "true" {
		//创建 token 并保存
		//需要加密的字符串为 UserName + UserPassword + 时间戳
		token := utils.GeneralToken(username + passwd)
		this.Ctx.Input.CruSession.Set("token", token)
		//在返回值 Header 里面设置 Token
		this.Ctx.Output.Context.ResponseWriter.Header().Set("X-Docker-Token", token)
		this.Ctx.Output.Context.ResponseWriter.Header().Set("WWW-Authenticate", token)
	}

	//		this.Ctx.Input.CruSession.Set("username", username)
	//		this.Ctx.Input.CruSession.Set("org", org)
	this.Ctx.Input.CruSession.Set("namespace", namespace)
	this.Ctx.Input.CruSession.Set("repository", repository)
	//在 SetSession 中增加读权限
	this.Ctx.Input.CruSession.Set("access", "read")
	//操作正常的输出
	this.Ctx.Output.Context.Output.SetStatus(http.StatusOK)
	this.Ctx.Output.Context.Output.Body([]byte(repo.JSON))

}

func (this *RepositoryAPIController) GetRepositoryTags() {

	isAuth, errCode, errInfo := models.DoAuthGetRepositoryTags(this.Ctx)

	if !isAuth {
		this.Ctx.Output.Context.Output.SetStatus(errCode)
		this.Ctx.Output.Context.Output.Body(errInfo)
		this.StopRun()
	}

	beego.Debug("[Namespace] " + this.Ctx.Input.Param(":namespace"))
	beego.Debug("[Repository] " + this.Ctx.Input.Param(":repo_name"))

	//username := this.Data["username"].(string)
	//	org := this.Ctx.Input.CruSession.Get("org").(string)

	namespace := this.Ctx.Input.Param(":namespace")
	repository := this.Ctx.Input.Param(":repo_name")

	//加密签名
	sign := ""
	if len(string(this.Ctx.Input.Header("X-Docker-Sign"))) > 0 {
		sign = string(this.Ctx.Input.Header("X-Docker-Sign"))
	}

	beego.Debug("[Sign] " + sign)

	repo := new(models.Repository)
	isHas, _, err := repo.Has(namespace, repository)
	if err != nil {
		beego.Error(fmt.Sprintf("[API 用户] 读取 %s/%s 的 Tags 数据错误: %s", namespace, repository, err.Error()))
		this.Ctx.Output.Context.Output.SetStatus(http.StatusBadRequest)
		this.Ctx.Output.Context.Output.Body([]byte("{\"错误\":\"读取 Tag 数据错误\"}"))
		this.StopRun()
	}

	if !isHas {
		beego.Error(fmt.Sprintf("[API 用户]  %s/%s 不存在", namespace, repository))
		this.Ctx.Output.Context.Output.SetStatus(http.StatusBadRequest)
		this.Ctx.Output.Context.Output.Body([]byte(fmt.Sprintf("[API 用户]  %s/%s 不存在", namespace, repository)))
		this.StopRun()
	}
	nowTags := "{"
	beego.Error("[API 用户] repo:::", repo)
	beego.Error("[API 用户] repo.Tags", repo.Tags)
	for index, value := range repo.Tags {
		if index != 0 {
			nowTags += ","

		}
		nowTag := new(models.Tag)
		fmt.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~", repo.Tags)
		err = nowTag.GetByUUID(value)
		if err != nil {
			beego.Error(fmt.Sprintf("[API 用户]  %s/%s Tags 不存在", namespace, repository))
			this.Ctx.Output.Context.Output.SetStatus(http.StatusBadRequest)
			this.Ctx.Output.Context.Output.Body([]byte(fmt.Sprintf("[API 用户]  %s/%s Tags 不存在", namespace, repository)))
			this.StopRun()
		}
		nowTags += fmt.Sprintf(`"%s":"%s"`, nowTag.Name, nowTag.ImageId)

	}
	nowTags += "}"
	//TODO 私有镜像仓库权限判断
	//操作正常的输出
	this.Ctx.Output.Context.Output.SetStatus(http.StatusOK)
	this.Ctx.Output.Context.Output.Body([]byte(nowTags))

}
