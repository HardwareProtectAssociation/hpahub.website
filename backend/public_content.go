package main

import (
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "path"
 "sort"
 "strings"
 "unicode/utf8"

 "github.com/pocketbase/dbx"
 "github.com/pocketbase/pocketbase/core"
 "github.com/pocketbase/pocketbase/tools/filesystem"
)

type contentInput struct { values map[string]json.RawMessage; files map[string][]*filesystem.File }

func readContentInput(e *core.RequestEvent, allowed map[string]bool) (*contentInput, error) {
 in := &contentInput{values: map[string]json.RawMessage{}, files: map[string][]*filesystem.File{}}
 if strings.HasPrefix(e.Request.Header.Get("Content-Type"), "multipart/form-data") {
  if err := e.Request.ParseMultipartForm(24 << 20); err != nil { return nil, err }
  for key, values := range e.Request.MultipartForm.Value {
   if !allowed[key] || key == "images" || key == "avatar" { return nil, fmt.Errorf("unknown field %s", key) }
   var value any
   if key == "keepImages" { value = values } else if len(values) != 1 { return nil, fmt.Errorf("duplicate field %s", key) } else if key == "published" || key == "profilePublic" || key == "removeAvatar" {
    if values[0] != "true" && values[0] != "false" { return nil, fmt.Errorf("invalid boolean") }; value = values[0] == "true"
   } else { value = values[0] }
   in.values[key], _ = json.Marshal(value)
  }
  for key, headers := range e.Request.MultipartForm.File {
   if !allowed[key] || (key != "images" && key != "avatar") { return nil, fmt.Errorf("unknown file field %s", key) }
   for _, header := range headers { file, err := filesystem.NewFileFromMultipart(header); if err != nil { return nil, err }; in.files[key] = append(in.files[key], file) }
  }
 } else {
  dec := json.NewDecoder(io.LimitReader(e.Request.Body, 1<<20))
  if err := dec.Decode(&in.values); err != nil { return nil, err }; var extra any; if err := dec.Decode(&extra); err != io.EOF { return nil, fmt.Errorf("invalid JSON") }
  for key := range in.values { if !allowed[key] || key == "images" || key == "avatar" { return nil, fmt.Errorf("unknown field %s", key) } }
 }
 return in, nil
}

func (in *contentInput) get(key string, value any) (bool, error) { raw, ok := in.values[key]; if !ok { return false, nil }; return true, json.Unmarshal(raw, value) }
func noCache(e *core.RequestEvent) { e.Response.Header().Set("Cache-Control", "no-store"); e.Response.Header().Set("X-Content-Type-Options", "nosniff") }
func requireContentRole(e *core.RequestEvent, admin, userOnly bool) error {
 if e.Auth == nil { return e.UnauthorizedError("Authentication required.", nil) }
 role := siteRole(e.Auth)
 if (admin && role != "admin" && role != "octopus") || (!admin && !internalRole(e.Auth)) || (userOnly && e.Auth.Collection().Name != "users") { return e.ForbiddenError("Insufficient permissions.", nil) }
 return nil
}
func contentURL(parts ...string) string { for i := range parts { parts[i] = url.PathEscape(parts[i]) }; return "/api/hpahub/" + strings.Join(parts, "/") }
func profileData(r *core.Record, public bool) map[string]any {
 avatar := ""; if r.GetString("avatar") != "" { if public { avatar = contentURL("members", r.Id, "avatar") } else { avatar = contentURL("me", "profile", "avatar") } }
 return map[string]any{"id":r.Id,"name":r.GetString("name"),"bio":r.GetString("bio"),"profilePublic":r.GetBool("profilePublic"),"avatarUrl":avatar}
}
func contentData(r *core.Record, base []string) map[string]any {
 names := r.GetStringSlice("images"); if names == nil { names = []string{} }; urls := make([]string, 0, len(names))
 for _, name := range names { parts := append(append([]string{}, base...), "images", name); urls = append(urls, contentURL(parts...)) }
 result := map[string]any{"id":r.Id,"title":r.GetString("title"),"text":r.GetString("text"),"images":names,"imageUrls":urls,"created":r.GetString("created"),"updated":r.GetString("updated")}
 if r.Collection().Name == "showcase_projects" { result["published"] = r.GetBool("published") }; return result
}
func records(e *core.RequestEvent, collection, filter string, params dbx.Params) ([]*core.Record, error) { return e.App.FindRecordsByFilter(collection, filter, "-created,-id", 0, 0, params) }
func visibleMember(e *core.RequestEvent, id string) (*core.Record, error) {
 r, err := e.App.FindRecordById("users", id); if err != nil || !r.GetBool("profilePublic") || (siteRole(r) != "member" && siteRole(r) != "admin") { return nil, e.NotFoundError("Not found.", nil) }; return r, nil
}
func publicMemberSkills(e *core.RequestEvent, member *core.Record) ([]map[string]any, error) {
 rows, err := e.App.FindRecordsByFilter("user_skills", "user_id = {:user}", "", 0, 0, dbx.Params{"user": member.Id}); if err != nil { return nil, err }
 skills, err := e.App.FindRecordsByFilter("skills", "", "name", 0, 0); if err != nil { return nil, err }
 byID := map[string]*core.Record{}; for _, skill := range skills { byID[skill.Id] = skill }
 viewer := ""
 if e.Auth != nil { if owner, isMember, err := personalCenterOwner(e.App, e.Auth); err == nil && isMember { viewer = owner.Id } }
 result := make([]map[string]any, 0, len(rows))
 for _, row := range rows {
  skill := byID[row.GetString("skill_id")]; if skill == nil { continue }
  count, liked, err := centerSkillLikeStats(e.App, row.Id, viewer, member.Id); if err != nil { return nil, err }
  imageNames := row.GetStringSlice("images"); images := make([]string, 0, len(imageNames)); for _, image := range imageNames { images = append(images, contentURL("skills", row.Id, "images", image)) }
  result = append(result, map[string]any{
   "userSkillId": row.Id,
   "name": skill.GetString("name"),
   "proficiency": centerSkillProficiency(row.GetString("proficiency")),
   "abilityNote": row.GetString("ability_note"),
   "images": images,
   "links": centerStrings(row.GetStringSlice("links")),
   "likeCount": count,
   "likeTier": centerLikeTier(count),
   "likedByMe": liked,
   "canEndorse": viewer != "" && viewer != member.Id,
  })
 }
 sort.SliceStable(result, func(i, j int) bool { return result[i]["name"].(string) < result[j]["name"].(string) })
 return result, nil
}
func serveContentFile(e *core.RequestEvent, r *core.Record, field, filename string) error {
 noCache(e); if filename == "" || path.Base(filename) != filename { return e.NotFoundError("Not found.", nil) }
 found := false; for _, name := range r.GetStringSlice(field) { if name == filename { found = true; break } }; if !found && r.GetString(field) == filename { found = true }; if !found { return e.NotFoundError("Not found.", nil) }
 fs, err := e.App.NewFilesystem(); if err != nil { return e.InternalServerError("File unavailable.", err) }; defer fs.Close()
 return fs.Serve(e.Response, e.Request, r.BaseFilesPath()+"/"+filename, filename)
}
func saveContent(e *core.RequestEvent, r *core.Record, project bool) error {
 allowed := map[string]bool{"title":true,"text":true,"images":true,"keepImages":true}; if project { allowed["published"] = true }
 in, err := readContentInput(e, allowed); if e.Request.MultipartForm != nil { defer e.Request.MultipartForm.RemoveAll() }; if err != nil { return e.BadRequestError("Invalid content.", err) }
 var title, text string; hasTitle, err := in.get("title", &title); if err != nil { return e.BadRequestError("Invalid title.", err) }; hasText, err := in.get("text", &text); if err != nil { return e.BadRequestError("Invalid text.", err) }
 if hasTitle { r.Set("title", strings.TrimSpace(title)) }; if hasText { r.Set("text", text) }
 if strings.TrimSpace(r.GetString("title")) == "" || utf8.RuneCountInString(r.GetString("title")) > 80 || utf8.RuneCountInString(r.GetString("text")) > 280 { return e.BadRequestError("Invalid content.", nil) }
 if project { var published bool; present, err := in.get("published", &published); if err != nil { return e.BadRequestError("Invalid published value.", err) }; if present { r.Set("published", published) } }
 keep := r.GetStringSlice("images"); var selected []string; present, err := in.get("keepImages", &selected); if err != nil { return e.BadRequestError("Invalid images.", err) }; if present { keep = selected } else if e.Request.MultipartForm != nil { keep = nil }
 existing := r.GetStringSlice("images"); seen := map[string]bool{}; values := make([]any, 0, len(keep)+len(in.files["images"]))
 for _, name := range keep { found := false; for _, old := range existing { if old == name { found = true; break } }; if !found || seen[name] { return e.BadRequestError("Invalid images.", nil) }; seen[name] = true; values = append(values, name) }
 for _, file := range in.files["images"] { values = append(values, file) }; if len(values) > 4 { return e.BadRequestError("Too many images.", nil) }; r.Set("images", values)
 if err := e.App.Save(r); err != nil { return e.BadRequestError("Unable to save content.", err) }; return nil
}
func bindPublicRoutes(s *core.ServeEvent) {
 profile := "/api/hpahub/me/profile"
 s.Router.GET(profile, func(e *core.RequestEvent) error { noCache(e); if err := requireContentRole(e,false,true); err != nil { return err }; return e.JSON(200, profileData(e.Auth,false)) })
 s.Router.PATCH(profile, func(e *core.RequestEvent) error {
  noCache(e); if err := requireContentRole(e,false,true); err != nil { return err }
  in, err := readContentInput(e,map[string]bool{"name":true,"bio":true,"profilePublic":true,"avatar":true,"removeAvatar":true}); if e.Request.MultipartForm != nil { defer e.Request.MultipartForm.RemoveAll() }; if err != nil { return e.BadRequestError("Invalid profile.",err) }
  r := e.Auth; var name,bio string; var public,remove bool
  if ok,err := in.get("name",&name); err != nil { return e.BadRequestError("Invalid name.",err) } else if ok { r.Set("name",strings.TrimSpace(name)) }
  if ok,err := in.get("bio",&bio); err != nil { return e.BadRequestError("Invalid bio.",err) } else if ok { r.Set("bio",bio) }
  if ok,err := in.get("profilePublic",&public); err != nil { return e.BadRequestError("Invalid visibility.",err) } else if ok { r.Set("profilePublic",public) }
  if _,err := in.get("removeAvatar",&remove); err != nil { return e.BadRequestError("Invalid avatar.",err) }
  files := in.files["avatar"]; if len(files)>1 || (remove && len(files)>0) { return e.BadRequestError("Invalid avatar.",nil) }; if remove { r.Set("avatar","") } else if len(files)==1 { r.Set("avatar",files[0]) }
  if utf8.RuneCountInString(r.GetString("name"))>80 || utf8.RuneCountInString(r.GetString("bio"))>600 || (r.GetBool("profilePublic") && strings.TrimSpace(r.GetString("name"))=="") { return e.BadRequestError("Invalid profile.",nil) }
  if err := e.App.Save(r); err != nil { return e.BadRequestError("Unable to save profile.",err) }; return e.JSON(200,profileData(r,false))
 })
 s.Router.GET(profile+"/avatar",func(e *core.RequestEvent) error { noCache(e); if err:=requireContentRole(e,false,true); err!=nil{return err}; return serveContentFile(e,e.Auth,"avatar",e.Auth.GetString("avatar")) })
 s.Router.GET("/api/hpahub/members",func(e *core.RequestEvent) error {
  noCache(e); users,err:=records(e,"users",`role = "member" || role = "admin"`,nil); if err!=nil{return e.InternalServerError("Unable to load members.",err)}; items:=[]any{}
  for _,r:=range users { if !r.GetBool("profilePublic") {items=append(items,map[string]any{"name":"匿名成员"})} else {p:=profileData(r,true); items=append(items,map[string]any{"id":r.Id,"name":r.GetString("name"),"avatarUrl":p["avatarUrl"]})} }; return e.JSON(200,map[string]any{"items":items})
 })
 s.Router.GET("/api/hpahub/members/{id}",func(e *core.RequestEvent) error {
  noCache(e); r,err:=visibleMember(e,e.Request.PathValue("id")); if err!=nil{return err}; cards,err:=records(e,"member_cards","owner = {:owner}",dbx.Params{"owner":r.Id}); if err!=nil{return e.InternalServerError("Unable to load cards.",err)}; list:=[]any{}; for _,c:=range cards { list=append(list,contentData(c,[]string{"members",r.Id,"cards",c.Id})) }; p:=profileData(r,true); delete(p,"profilePublic"); p["cards"]=list; skills,err:=publicMemberSkills(e,r); if err!=nil{return e.InternalServerError("Unable to load skills.",err)}; p["skills"]=skills; return e.JSON(200,p)
 })
 s.Router.GET("/api/hpahub/members/{id}/avatar",func(e *core.RequestEvent) error { noCache(e); r,err:=visibleMember(e,e.Request.PathValue("id")); if err!=nil{return err}; return serveContentFile(e,r,"avatar",r.GetString("avatar")) })
 s.Router.GET("/api/hpahub/members/{id}/cards/{card}/images/{file}",func(e *core.RequestEvent) error { noCache(e); r,err:=visibleMember(e,e.Request.PathValue("id")); if err!=nil{return err}; card,err:=e.App.FindRecordById("member_cards",e.Request.PathValue("card")); if err!=nil || card.GetString("owner")!=r.Id{return e.NotFoundError("Not found.",nil)}; return serveContentFile(e,card,"images",e.Request.PathValue("file")) })
 bindContentCollection(s,false); bindContentCollection(s,true)
 s.Router.GET("/api/hpahub/showcase",func(e *core.RequestEvent) error { noCache(e); items,err:=records(e,"showcase_projects","published = true",nil); if err!=nil{return e.InternalServerError("Unable to load showcase.",err)}; out:=[]any{}; for _,r:=range items {out=append(out,contentData(r,[]string{"showcase",r.Id}))}; return e.JSON(200,map[string]any{"items":out}) })
 s.Router.GET("/api/hpahub/showcase/{id}",func(e *core.RequestEvent) error { noCache(e); r,err:=e.App.FindRecordById("showcase_projects",e.Request.PathValue("id")); if err!=nil || !r.GetBool("published") {return e.NotFoundError("Not found.",nil)}; return e.JSON(200,contentData(r,[]string{"showcase",r.Id})) })
 s.Router.GET("/api/hpahub/showcase/{id}/images/{file}",func(e *core.RequestEvent) error { noCache(e); r,err:=e.App.FindRecordById("showcase_projects",e.Request.PathValue("id")); if err!=nil || !r.GetBool("published") {return e.NotFoundError("Not found.",nil)}; return serveContentFile(e,r,"images",e.Request.PathValue("file")) })
}
func bindContentCollection(s *core.ServeEvent, project bool) {
 base,collection := "/api/hpahub/me/cards","member_cards"; parts:=[]string{"me","cards"}; if project {base,collection="/api/hpahub/manage/showcase","showcase_projects";parts=[]string{"manage","showcase"} }
 authorize:=func(e *core.RequestEvent) error {return requireContentRole(e,project,!project)}
 find:=func(e *core.RequestEvent)(*core.Record,error) { r,err:=e.App.FindRecordById(collection,e.Request.PathValue("id")); if err!=nil || (!project && r.GetString("owner")!=e.Auth.Id) {return nil,e.NotFoundError("Not found.",nil)}; return r,nil }
 data:=func(r *core.Record) map[string]any {return contentData(r,append(append([]string{},parts...),r.Id))}
 s.Router.GET(base,func(e *core.RequestEvent) error { noCache(e); if err:=authorize(e);err!=nil{return err}; filter:=""; params:=dbx.Params{}; if !project {filter="owner = {:owner}";params["owner"]=e.Auth.Id}; rows,err:=records(e,collection,filter,params);if err!=nil{return e.InternalServerError("Unable to load content.",err)};out:=[]any{};for _,r:=range rows {out=append(out,data(r))};return e.JSON(200,map[string]any{"items":out}) })
 s.Router.POST(base,func(e *core.RequestEvent) error { noCache(e);if err:=authorize(e);err!=nil{return err};c,err:=e.App.FindCollectionByNameOrId(collection);if err!=nil{return e.InternalServerError("Collection unavailable.",err)};r:=core.NewRecord(c);if !project {r.Set("owner",e.Auth.Id)};if err:=saveContent(e,r,project);err!=nil{return err};return e.JSON(200,data(r)) })
 s.Router.PATCH(base+"/{id}",func(e *core.RequestEvent) error { noCache(e);if err:=authorize(e);err!=nil{return err};r,err:=find(e);if err!=nil{return err};if err:=saveContent(e,r,project);err!=nil{return err};return e.JSON(200,data(r)) })
 s.Router.DELETE(base+"/{id}",func(e *core.RequestEvent) error { noCache(e);if err:=authorize(e);err!=nil{return err};r,err:=find(e);if err!=nil{return err};if err=e.App.Delete(r);err!=nil{return e.InternalServerError("Unable to delete content.",err)};return e.NoContent(http.StatusNoContent) })
 s.Router.GET(base+"/{id}/images/{file}",func(e *core.RequestEvent) error { noCache(e);if err:=authorize(e);err!=nil{return err};r,err:=find(e);if err!=nil{return err};return serveContentFile(e,r,"images",e.Request.PathValue("file")) })
}
