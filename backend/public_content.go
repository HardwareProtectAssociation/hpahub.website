package main

import (
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "path"
 "sort"
 "strconv"
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
   if !allowed[key] || key == "images" || key == "avatar" || key == "avatarThumb" { return nil, fmt.Errorf("unknown field %s", key) }
   var value any
   if key == "keepImages" { value = values } else if len(values) != 1 { return nil, fmt.Errorf("duplicate field %s", key) } else if key == "published" || key == "profilePublic" || key == "removeAvatar" {
    if values[0] != "true" && values[0] != "false" { return nil, fmt.Errorf("invalid boolean") }; value = values[0] == "true"
   } else { value = values[0] }
   in.values[key], _ = json.Marshal(value)
  }
  for key, headers := range e.Request.MultipartForm.File {
   if !allowed[key] || (key != "images" && key != "avatar" && key != "avatarThumb") { return nil, fmt.Errorf("unknown file field %s", key) }
   for _, header := range headers { file, err := filesystem.NewFileFromMultipart(header); if err != nil { return nil, err }; in.files[key] = append(in.files[key], file) }
  }
 } else {
  dec := json.NewDecoder(io.LimitReader(e.Request.Body, 1<<20))
  if err := dec.Decode(&in.values); err != nil { return nil, err }; var extra any; if err := dec.Decode(&extra); err != io.EOF { return nil, fmt.Errorf("invalid JSON") }
  for key := range in.values { if !allowed[key] || key == "images" || key == "avatar" || key == "avatarThumb" { return nil, fmt.Errorf("unknown field %s", key) } }
 }
 return in, nil
}

func (in *contentInput) get(key string, value any) (bool, error) { raw, ok := in.values[key]; if !ok { return false, nil }; return true, json.Unmarshal(raw, value) }
func setAvatar(r *core.Record, input *contentInput, remove bool) error {
 files, thumbs := input.files["avatar"], input.files["avatarThumb"]
 if len(files) > 1 || len(thumbs) > 1 || (remove && (len(files) > 0 || len(thumbs) > 0)) || (len(thumbs) > 0 && len(files) == 0) { return fmt.Errorf("invalid avatar") }
 if remove { r.Set("avatar", ""); r.Set("avatar_thumb", ""); r.Set("avatar_cropped", "") } else if len(files) == 1 {
  r.Set("avatar", files[0]); r.Set("avatar_thumb", ""); r.Set("avatar_cropped", "")
  if len(thumbs) == 1 { r.Set("avatar_thumb", thumbs[0]) }
 }
 return nil
}
func serveAvatar(e *core.RequestEvent, r *core.Record) error {
 field := "avatar"; if r.GetString("avatar_thumb") != "" { field = "avatar_thumb" }
 return serveContentFile(e, r, field, r.GetString(field))
}
func noCache(e *core.RequestEvent) { e.Response.Header().Set("Cache-Control", "no-store"); e.Response.Header().Set("X-Content-Type-Options", "nosniff") }
func requireContentRole(e *core.RequestEvent, admin, userOnly bool) error {
 if e.Auth == nil { return e.UnauthorizedError("Authentication required.", nil) }
 role := siteRole(e.Auth)
 if (admin && role != "admin" && role != "octopus") || (!admin && !internalRole(e.Auth)) || (userOnly && e.Auth.Collection().Name != "users") { return e.ForbiddenError("Insufficient permissions.", nil) }
 return nil
}
func contentURL(parts ...string) string { for i := range parts { parts[i] = url.PathEscape(parts[i]) }; return "/api/hpahub/" + strings.Join(parts, "/") }
var profileSections = map[string]bool{"avatar":true, "bio":true, "status":true, "cards":true, "identities":true, "projects":true, "career":true, "skills":true}
func profileSectionHidden(r *core.Record, section string) bool {
 for _, value := range r.GetStringSlice("hidden_profile_sections") { if value == section { return true } }; return false
}
func profileData(r *core.Record, public bool) map[string]any {
 avatar := ""; if r.GetString("avatar") != "" && (!public || !profileSectionHidden(r,"avatar")) { if public { avatar = contentURL("members", r.Id, "avatar") } else { avatar = contentURL("me", "profile", "avatar") } }
 bio, status := r.GetString("bio"), r.GetString("current_status")
 if public && profileSectionHidden(r,"bio") { bio = "" }; if public && profileSectionHidden(r,"status") { status = "" }
 return map[string]any{"id":r.Id,"name":r.GetString("name"),"bio":bio,"profilePublic":r.GetBool("profilePublic"),"avatarUrl":avatar,"currentStatus":status}
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
func publicMemberIdentities(e *core.RequestEvent, member *core.Record) ([]map[string]any, error) {
 if profileSectionHidden(member,"identities") { return []map[string]any{}, nil }
 rows, err := e.App.FindRecordsByFilter("user_identities", "user_id = {:user} && featured = true", "", 0, 0, dbx.Params{"user": member.Id}); if err != nil { return nil, err }
 identities, err := e.App.FindRecordsByFilter("collaboration_identities", "", "category,name", 0, 0); if err != nil { return nil, err }
 byID := map[string]*core.Record{}; for _, identity := range identities { byID[identity.Id] = identity }
 result := make([]map[string]any, 0, len(rows))
 for _, row := range rows {
  identity := byID[row.GetString("identity_id")]; if identity == nil { continue }
  result = append(result, map[string]any{"id": identity.Id, "name": identity.GetString("name"), "category": identity.GetString("category")})
 }
 return result, nil
}
func publicMemberProjects(e *core.RequestEvent, member *core.Record) ([]publicProject, []publicProject, error) {
 if profileSectionHidden(member,"projects") { return []publicProject{}, []publicProject{}, nil }
 projects := map[string]*core.Record{}
 owned, err := e.App.FindRecordsByFilter("projects", `owner_user_id = {:user} && review_status = "approved" && deleted_at = ""`, "-updated", 0, 0, dbx.Params{"user": member.Id}); if err != nil { return nil, nil, err }
 for _, project := range owned { projects[project.Id] = project }
 memberships, err := e.App.FindRecordsByFilter("project_members", `user_id = {:user} && status = "active"`, "", 0, 0, dbx.Params{"user": member.Id}); if err != nil { return nil, nil, err }
 for _, membership := range memberships {
  project, err := e.App.FindRecordById("projects", membership.GetString("project_id")); if err != nil { continue }
  if project.GetString("review_status") != "approved" || project.GetString("deleted_at") != "" { continue }
  projects[project.Id] = project
 }
 completed := []publicProject{}; ongoing := []publicProject{}; owners := map[string]*core.Record{}
 for _, project := range projects {
  switch project.GetString("execution_status") {
  case "completed": completed = append(completed, projectSummary(e, project, owners))
  case "ongoing": ongoing = append(ongoing, projectSummary(e, project, owners))
  }
 }
 sort.SliceStable(completed, func(i, j int) bool { return completed[i].Updated > completed[j].Updated })
 sort.SliceStable(ongoing, func(i, j int) bool { return ongoing[i].Updated > ongoing[j].Updated })
 return completed, ongoing, nil
}
func publicMemberCareer(e *core.RequestEvent, member *core.Record) ([]centerBadge, error) {
 if profileSectionHidden(member,"career") { return []centerBadge{}, nil }
 identities, err := centerIdentityRecords(e.App); if err != nil { return nil, err }
 return centerBadges(e.App, member.Id, identities)
}
func publicMemberRelations(e *core.RequestEvent, member *core.Record) (memberRelationSummary, error) {
 viewerID := ""
 if e.Auth != nil {
  if owner, ok, err := personalCenterOwner(e.App, e.Auth); err == nil && ok { viewerID = owner.Id }
 }
 return memberRelationSummaryData(e.App, member.Id, viewerID)
}
func publicMemberSkills(e *core.RequestEvent, member *core.Record) ([]map[string]any, error) {
 if profileSectionHidden(member,"skills") { return []map[string]any{}, nil }
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
   "featured": row.GetBool("featured"),
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
 e.Response.Header().Set("Cache-Control", "private, no-cache")
 e.Response.Header().Set("ETag", strconv.Quote(r.Id + "/" + filename))
 e.Response.Header().Add("Vary", "Authorization")
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
 s.Router.GET("/api/hpahub/me/profile-visibility",func(e *core.RequestEvent) error {
  noCache(e); if err:=requireContentRole(e,false,false); err!=nil{return err}
  owner,member,err:=personalCenterOwner(e.App,e.Auth); if err!=nil {return e.InternalServerError("Unable to load profile.",err)}; if !member {return e.ForbiddenError("No member profile is linked to this account.",nil)}
  return e.JSON(200,map[string]any{"profilePublic":owner.GetBool("profilePublic"),"hiddenSections":owner.GetStringSlice("hidden_profile_sections")})
 })
 s.Router.PUT("/api/hpahub/me/profile-visibility",func(e *core.RequestEvent) error {
  noCache(e); if err:=requireContentRole(e,false,false); err!=nil{return err}
  owner,member,err:=personalCenterOwner(e.App,e.Auth); if err!=nil {return e.InternalServerError("Unable to load profile.",err)}; if !member {return e.ForbiddenError("No member profile is linked to this account.",nil)}
  in,err:=readContentInput(e,map[string]bool{"profilePublic":true,"hiddenSections":true}); if err!=nil{return e.BadRequestError("Invalid visibility.",err)}
  var public bool; var hidden []string
  a,err:=in.get("profilePublic",&public); if err!=nil || !a {return e.BadRequestError("Invalid visibility.",err)}
  b,err:=in.get("hiddenSections",&hidden); if err!=nil || !b || hidden==nil {return e.BadRequestError("Invalid visibility.",err)}
  seen:=map[string]bool{}; for _,section:=range hidden {if !profileSections[section] || seen[section] {return e.BadRequestError("Invalid visibility.",nil)}; seen[section]=true}
  if public && strings.TrimSpace(owner.GetString("name"))=="" {return e.BadRequestError("Invalid profile.",nil)}
  owner.Set("profilePublic",public); owner.Set("hidden_profile_sections",hidden)
  if err:=e.App.Save(owner); err!=nil{return e.BadRequestError("Unable to save profile.",err)}
  return e.JSON(200,map[string]any{"profilePublic":public,"hiddenSections":hidden})
 })
 s.Router.GET(profile, func(e *core.RequestEvent) error { noCache(e); if err := requireContentRole(e,false,true); err != nil { return err }; return e.JSON(200, profileData(e.Auth,false)) })
 s.Router.PATCH(profile, func(e *core.RequestEvent) error {
  noCache(e); if err := requireContentRole(e,false,true); err != nil { return err }
  in, err := readContentInput(e,map[string]bool{"name":true,"bio":true,"profilePublic":true,"avatar":true,"avatarThumb":true,"removeAvatar":true}); if e.Request.MultipartForm != nil { defer e.Request.MultipartForm.RemoveAll() }; if err != nil { return e.BadRequestError("Invalid profile.",err) }
  r := e.Auth; var name,bio string; var public,remove bool
  if ok,err := in.get("name",&name); err != nil { return e.BadRequestError("Invalid name.",err) } else if ok { r.Set("name",strings.TrimSpace(name)) }
  if ok,err := in.get("bio",&bio); err != nil { return e.BadRequestError("Invalid bio.",err) } else if ok { r.Set("bio",bio) }
  if ok,err := in.get("profilePublic",&public); err != nil { return e.BadRequestError("Invalid visibility.",err) } else if ok { r.Set("profilePublic",public) }
  if _,err := in.get("removeAvatar",&remove); err != nil { return e.BadRequestError("Invalid avatar.",err) }
  if err := setAvatar(r, in, remove); err != nil { return e.BadRequestError("Invalid avatar.",err) }
  if utf8.RuneCountInString(r.GetString("name"))>80 || utf8.RuneCountInString(r.GetString("bio"))>600 || (r.GetBool("profilePublic") && strings.TrimSpace(r.GetString("name"))=="") { return e.BadRequestError("Invalid profile.",nil) }
  if err := e.App.Save(r); err != nil { return e.BadRequestError("Unable to save profile.",err) }; return e.JSON(200,profileData(r,false))
 })
 s.Router.GET(profile+"/avatar",func(e *core.RequestEvent) error { noCache(e); if err:=requireContentRole(e,false,true); err!=nil{return err}; return serveAvatar(e,e.Auth) })
 s.Router.GET("/api/hpahub/members",func(e *core.RequestEvent) error {
  noCache(e); users,err:=records(e,"users",`role = "member" || role = "admin"`,nil); if err!=nil{return e.InternalServerError("Unable to load members.",err)}; items:=[]any{}
  for _,r:=range users { if !r.GetBool("profilePublic") {items=append(items,map[string]any{"name":"匿名成员"})} else {p:=profileData(r,true); items=append(items,map[string]any{"id":r.Id,"name":r.GetString("name"),"avatarUrl":p["avatarUrl"]})} }; return e.JSON(200,map[string]any{"items":items})
 })
 s.Router.GET("/api/hpahub/members/{id}",func(e *core.RequestEvent) error {
  noCache(e); r,err:=visibleMember(e,e.Request.PathValue("id")); if err!=nil{return err}; cards,err:=records(e,"member_cards","owner = {:owner}",dbx.Params{"owner":r.Id}); if err!=nil{return e.InternalServerError("Unable to load cards.",err)}; list:=[]any{}; for _,c:=range cards { list=append(list,contentData(c,[]string{"members",r.Id,"cards",c.Id})) }; p:=profileData(r,true); delete(p,"profilePublic"); p["hiddenSections"]=r.GetStringSlice("hidden_profile_sections"); if profileSectionHidden(r,"cards") {list=[]any{}}; p["cards"]=list; skills,err:=publicMemberSkills(e,r); if err!=nil{return e.InternalServerError("Unable to load skills.",err)}; p["skills"]=skills; identities,err:=publicMemberIdentities(e,r); if err!=nil{return e.InternalServerError("Unable to load identities.",err)}; p["identities"]=identities; completed,ongoing,err:=publicMemberProjects(e,r); if err!=nil{return e.InternalServerError("Unable to load projects.",err)}; p["projectsCompleted"]=completed; p["projectsOngoing"]=ongoing; career,err:=publicMemberCareer(e,r); if err!=nil{return e.InternalServerError("Unable to load career.",err)}; p["career"]=career; relations,err:=publicMemberRelations(e,r); if err!=nil{return e.InternalServerError("Unable to load relations.",err)}; p["relations"]=relations; return e.JSON(200,p)
 })
 s.Router.GET("/api/hpahub/members/{id}/avatar",func(e *core.RequestEvent) error { noCache(e); r,err:=visibleMember(e,e.Request.PathValue("id")); if err!=nil{return err}; if profileSectionHidden(r,"avatar") {return e.NotFoundError("Not found.",nil)}; return serveAvatar(e,r) })
 s.Router.GET("/api/hpahub/members/{id}/cards/{card}/images/{file}",func(e *core.RequestEvent) error { noCache(e); r,err:=visibleMember(e,e.Request.PathValue("id")); if err!=nil{return err}; if profileSectionHidden(r,"cards") {return e.NotFoundError("Not found.",nil)}; card,err:=e.App.FindRecordById("member_cards",e.Request.PathValue("card")); if err!=nil || card.GetString("owner")!=r.Id{return e.NotFoundError("Not found.",nil)}; return serveContentFile(e,card,"images",e.Request.PathValue("file")) })
 bindContentCollection(s,false); bindContentCollection(s,true)
 s.Router.GET("/api/hpahub/showcase",func(e *core.RequestEvent) error { noCache(e); items,err:=records(e,"showcase_projects","published = true",nil); if err!=nil{return e.InternalServerError("Unable to load showcase.",err)}; out:=[]any{}; for _,r:=range items {out=append(out,contentData(r,[]string{"showcase",r.Id}))}; return e.JSON(200,map[string]any{"items":out}) })
 s.Router.GET("/api/hpahub/showcase/{id}",func(e *core.RequestEvent) error { noCache(e); r,err:=e.App.FindRecordById("showcase_projects",e.Request.PathValue("id")); if err!=nil || !r.GetBool("published") {return e.NotFoundError("Not found.",nil)}; return e.JSON(200,contentData(r,[]string{"showcase",r.Id})) })
 s.Router.GET("/api/hpahub/showcase/{id}/images/{file}",func(e *core.RequestEvent) error { noCache(e); r,err:=e.App.FindRecordById("showcase_projects",e.Request.PathValue("id")); if err!=nil || !r.GetBool("published") {return e.NotFoundError("Not found.",nil)}; return serveContentFile(e,r,"images",e.Request.PathValue("file")) })
}
func bindContentCollection(s *core.ServeEvent, project bool) {
 base,collection := "/api/hpahub/me/cards","member_cards"; parts:=[]string{"me","cards"}; if project {base,collection="/api/hpahub/manage/showcase","showcase_projects";parts=[]string{"manage","showcase"} }
 authorize:=func(e *core.RequestEvent) error {return requireContentRole(e,project,false)}
 ownerID:=func(e *core.RequestEvent)(string,error) {
  if project {return "",nil}
  owner,member,err:=personalCenterOwner(e.App,e.Auth)
  if err!=nil {return "",e.InternalServerError("Unable to load profile.",err)}
  if !member {return "",e.ForbiddenError("No member profile is linked to this account.",nil)}
  return owner.Id,nil
 }
 find:=func(e *core.RequestEvent)(*core.Record,error) { r,err:=e.App.FindRecordById(collection,e.Request.PathValue("id")); if err!=nil {return nil,e.NotFoundError("Not found.",nil)}; if !project {owner,err:=ownerID(e); if err!=nil {return nil,err}; if r.GetString("owner")!=owner {return nil,e.NotFoundError("Not found.",nil)}}; return r,nil }
 data:=func(r *core.Record) map[string]any {return contentData(r,append(append([]string{},parts...),r.Id))}
 s.Router.GET(base,func(e *core.RequestEvent) error { noCache(e); if err:=authorize(e);err!=nil{return err}; filter:=""; params:=dbx.Params{}; if !project {owner,err:=ownerID(e); if err!=nil{return err}; filter="owner = {:owner}";params["owner"]=owner}; rows,err:=records(e,collection,filter,params);if err!=nil{return e.InternalServerError("Unable to load content.",err)};out:=[]any{};for _,r:=range rows {out=append(out,data(r))};return e.JSON(200,map[string]any{"items":out}) })
 s.Router.POST(base,func(e *core.RequestEvent) error { noCache(e);if err:=authorize(e);err!=nil{return err};c,err:=e.App.FindCollectionByNameOrId(collection);if err!=nil{return e.InternalServerError("Collection unavailable.",err)};r:=core.NewRecord(c);if !project {owner,err:=ownerID(e); if err!=nil{return err}; r.Set("owner",owner)};if err:=saveContent(e,r,project);err!=nil{return err};return e.JSON(200,data(r)) })
 s.Router.PATCH(base+"/{id}",func(e *core.RequestEvent) error { noCache(e);if err:=authorize(e);err!=nil{return err};r,err:=find(e);if err!=nil{return err};if err:=saveContent(e,r,project);err!=nil{return err};return e.JSON(200,data(r)) })
 s.Router.DELETE(base+"/{id}",func(e *core.RequestEvent) error { noCache(e);if err:=authorize(e);err!=nil{return err};r,err:=find(e);if err!=nil{return err};if err=e.App.Delete(r);err!=nil{return e.InternalServerError("Unable to delete content.",err)};return e.NoContent(http.StatusNoContent) })
 s.Router.GET(base+"/{id}/images/{file}",func(e *core.RequestEvent) error { noCache(e);if err:=authorize(e);err!=nil{return err};r,err:=find(e);if err!=nil{return err};return serveContentFile(e,r,"images",e.Request.PathValue("file")) })
}
