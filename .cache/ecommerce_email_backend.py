from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api')
def put(name,s):
 p=api/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(s,encoding='utf-8')
put('models/email_campaign.go','''package models
import("time";"github.com/google/uuid";"gorm.io/gorm")
type EmailCampaign struct{ID string `json:"id" gorm:"primaryKey;type:char(36)"`;Subject string `json:"subject"`;Body string `json:"body"`;Status string `json:"status"`;ScheduledAt *time.Time `json:"scheduledAt"`;CreatedBy string `json:"createdBy"`;CreatedAt time.Time `json:"createdAt"`;UpdatedAt time.Time `json:"updatedAt"`}
func(c *EmailCampaign)BeforeCreate(tx *gorm.DB)error{if c.ID==""{c.ID=uuid.NewString()};return nil}
type EmailDelivery struct{ID string `json:"id" gorm:"primaryKey;type:char(36)"`;CampaignID string `json:"campaignId" gorm:"uniqueIndex:campaign_recipient"`;UserID string `json:"userId" gorm:"uniqueIndex:campaign_recipient"`;Email string `json:"email"`;Status string `json:"status"`;Error string `json:"error,omitempty"`;SentAt *time.Time `json:"sentAt"`;CreatedAt time.Time `json:"createdAt"`;UpdatedAt time.Time `json:"updatedAt"`}
func(d *EmailDelivery)BeforeCreate(tx *gorm.DB)error{if d.ID==""{d.ID=uuid.NewString()};return nil}
type EmailPreference struct{UserID string `json:"userId" gorm:"primaryKey;type:char(36)"`;Token string `json:"-" gorm:"uniqueIndex"`;Unsubscribed bool `json:"unsubscribed"`;UpdatedAt time.Time}
type EmailAutomation struct{ID uint `gorm:"primaryKey"`;Enabled bool `json:"enabled"`;UpdatedBy string;UpdatedAt time.Time}
''')
put('services/email_campaigns.go','''package services
import("html";"log";"strings";"time";"afrigoals-api.com/models";"github.com/google/uuid";"gorm.io/gorm";"gorm.io/gorm/clause")
func RunEmailAutomation(db *gorm.DB,baseURL string){
 ticker:=time.NewTicker(time.Minute);defer ticker.Stop()
 for range ticker.C{var config models.EmailAutomation;if db.First(&config,"id = ?",1).Error!=nil||!config.Enabled{continue};sender,err:=NewEmailService();if err!=nil{continue};if err:=ProcessEmailCampaigns(db,sender,baseURL);err!=nil{log.Print("email campaign worker could not complete a batch")}}
}
// Claims and provider idempotency keys protect a scheduled campaign from duplicate sends.
// Failed or ambiguous sends are recorded for admin review, never retried blindly.
func ProcessEmailCampaigns(db *gorm.DB,sender *EmailService,baseURL string)error{
 var config models.EmailAutomation;if err:=db.First(&config,"id = ?",1).Error;err!=nil||!config.Enabled{return err}
 var campaigns []models.EmailCampaign
 if err:=db.Where("status IN ? AND scheduled_at <= ?",[]string{"scheduled","sending"},time.Now()).Order("scheduled_at").Limit(3).Find(&campaigns).Error;err!=nil{return err}
 for _,campaign:=range campaigns{
  if err:=db.Model(&campaign).Update("status","sending").Error;err!=nil{return err}
  var users []models.User;if err:=db.Where("email_verified = ? AND role IN ?",true,[]string{"user","vendor"}).Find(&users).Error;err!=nil{return err}
  for _,user:=range users{
   pref:=models.EmailPreference{UserID:user.ID,Token:uuid.NewString()};if err:=db.Where("user_id = ?",user.ID).FirstOrCreate(&pref).Error;err!=nil{return err};if pref.Unsubscribed{continue}
   delivery:=models.EmailDelivery{CampaignID:campaign.ID,UserID:user.ID,Email:user.Email,Status:"pending"}
   if err:=db.Clauses(clause.OnConflict{Columns:[]clause.Column{{Name:"campaign_id"},{Name:"user_id"}},DoNothing:true}).Create(&delivery).Error;err!=nil{return err}
  }
  var deliveries []models.EmailDelivery;if err:=db.Where("campaign_id = ? AND status = ?",campaign.ID,"pending").Limit(100).Find(&deliveries).Error;err!=nil{return err}
  for _,delivery:=range deliveries{
   claim:=db.Model(&models.EmailDelivery{}).Where("id = ? AND status = ?",delivery.ID,"pending").Update("status","sending");if claim.Error!=nil{return claim.Error};if claim.RowsAffected==0{continue}
   var pref models.EmailPreference;if err:=db.First(&pref,"user_id = ?",delivery.UserID).Error;err!=nil{return err}
   if pref.Unsubscribed{if err:=db.Model(&delivery).Update("status","unsubscribed").Error;err!=nil{return err};continue}
   unsubscribe:=strings.TrimRight(baseURL,"/")+"/api/v1/email/unsubscribe/"+pref.Token
   body:="<h1>"+html.EscapeString(campaign.Subject)+"</h1><p>"+strings.ReplaceAll(html.EscapeString(campaign.Body),"\\n","<br>")+"</p><p><a href=\\\""+html.EscapeString(unsubscribe)+"\\\">Unsubscribe from marketing emails</a></p>"
   err:=sender.sendHTMLWithKey(delivery.Email,campaign.Subject,body,"campaign-"+delivery.ID)
   updates:=map[string]any{"status":"sent","sent_at":time.Now(),"error":""};if err!=nil{updates=map[string]any{"status":"failed","error":"Delivery failed or outcome unknown; review before retrying."}}
   if err:=db.Model(&delivery).Updates(updates).Error;err!=nil{return err}
  }
  var remaining int64;if err:=db.Model(&models.EmailDelivery{}).Where("campaign_id = ? AND status IN ?",campaign.ID,[]string{"pending","sending"}).Count(&remaining).Error;err!=nil{return err}
  if remaining==0{if err:=db.Model(&campaign).Update("status","completed").Error;err!=nil{return err}}
 }
 return nil
}
''')
p=api/'services/email.go';s=p.read_text(encoding='utf-8');needle='func (s *EmailService) sendHTML(to string, subject string, htmlBody string) error {';assert needle in s
s=s.replace(needle,needle+'\n return s.sendHTMLWithKey(to,subject,htmlBody,"")\n}\nfunc (s *EmailService) sendHTMLWithKey(to,subject,htmlBody,key string) error {',1)
s=s.replace('req.Header.Set("Authorization", "Bearer "+s.APIKey)','if key!=""{req.Header.Set("Idempotency-Key",key)}\n req.Header.Set("Authorization", "Bearer "+s.APIKey)',1);put('services/email.go',s)
put('routes/email_campaigns.go','''package routes
import("errors";"html";"os";"strings";"time";"afrigoals-api.com/models";"afrigoals-api.com/middleware";"github.com/gofiber/fiber/v2";"gorm.io/gorm";"gorm.io/gorm/clause")
func RegisterEmailCampaigns(router fiber.Router,auth fiber.Handler,deps Deps){
 router.Get("/admin/email-automation",auth,middleware.RequireAdmin(),func(c *fiber.Ctx)error{
  var config models.EmailAutomation;err:=deps.DB.First(&config,"id = ?",1).Error;if err!=nil&&!errors.Is(err,gorm.ErrRecordNotFound){return c.Status(500).JSON(fiber.Map{"error":"could not load automation settings"})}
  return c.JSON(fiber.Map{"enabled":config.Enabled,"providerConfigured":os.Getenv("RESEND_API_KEY")!=""&&os.Getenv("RESEND_FROM_EMAIL")!=""})
 })
 router.Put("/admin/email-automation",auth,middleware.RequireAdmin(),func(c *fiber.Ctx)error{
  var input struct{Enabled bool `json:"enabled"`};if c.BodyParser(&input)!=nil{return c.Status(400).JSON(fiber.Map{"error":"invalid automation settings"})}
  userID,_:=c.Locals("userID").(string);row:=models.EmailAutomation{ID:1,Enabled:input.Enabled,UpdatedBy:userID}
  if err:=deps.DB.Clauses(clause.OnConflict{Columns:[]clause.Column{{Name:"id"}},DoUpdates:clause.AssignmentColumns([]string{"enabled","updated_by","updated_at"})}).Create(&row).Error;err!=nil{return c.Status(500).JSON(fiber.Map{"error":"could not save automation settings"})};return c.JSON(fiber.Map{"enabled":input.Enabled})
 })
 router.Get("/admin/email-campaigns",auth,middleware.RequireAdmin(),func(c *fiber.Ctx)error{
  var campaigns []models.EmailCampaign;if err:=deps.DB.Order("created_at DESC").Limit(100).Find(&campaigns).Error;err!=nil{return c.Status(500).JSON(fiber.Map{"error":"could not load campaigns"})};return c.JSON(campaigns)
 })
 router.Put("/admin/email-campaigns",auth,middleware.RequireAdmin(),func(c *fiber.Ctx)error{
  var input models.EmailCampaign;if c.BodyParser(&input)!=nil||strings.TrimSpace(input.Subject)==""||strings.TrimSpace(input.Body)==""||len(input.Subject)>200||len(input.Body)>20000{return c.Status(400).JSON(fiber.Map{"error":"subject and message are required (max 200 / 20000 characters)"})}
  if input.Status!="draft"&&input.Status!="scheduled"{return c.Status(400).JSON(fiber.Map{"error":"save a draft or scheduled campaign"})}
  if input.Status=="scheduled"&&(input.ScheduledAt==nil||input.ScheduledAt.Before(time.Now())){return c.Status(400).JSON(fiber.Map{"error":"choose a future send time"})}
  if input.ID!=""{var old models.EmailCampaign;if deps.DB.First(&old,"id = ?",input.ID).Error!=nil{return c.SendStatus(404)};if old.Status!="draft"&&old.Status!="scheduled"{return c.Status(409).JSON(fiber.Map{"error":"campaign has already started"})}}
  userID,_:=c.Locals("userID").(string);input.CreatedBy=userID
  if err:=deps.DB.Save(&input).Error;err!=nil{return c.Status(500).JSON(fiber.Map{"error":"could not save campaign"})};return c.JSON(input)
 })
 router.Get("/admin/email-campaigns/:id/deliveries",auth,middleware.RequireAdmin(),func(c *fiber.Ctx)error{var deliveries []models.EmailDelivery;if err:=deps.DB.Where("campaign_id = ?",c.Params("id")).Order("created_at DESC").Limit(1000).Find(&deliveries).Error;err!=nil{return c.Status(500).JSON(fiber.Map{"error":"could not load delivery statuses"})};return c.JSON(deliveries)})
 router.Get("/email/unsubscribe/:token",func(c *fiber.Ctx)error{return c.Type("html").SendString("<h1>AfriGoals email preferences</h1><p>Stop marketing emails. Order and security updates are unaffected.</p><form method=\\\"post\\\"><button type=\\\"submit\\\">Unsubscribe</button></form>")})
 router.Post("/email/unsubscribe/:token",func(c *fiber.Ctx)error{token:=strings.TrimSpace(c.Params("token"));if len(token)!=36{return c.SendStatus(404)};result:=deps.DB.Model(&models.EmailPreference{}).Where("token = ?",token).Update("unsubscribed",true);if result.Error!=nil{return c.SendStatus(500)};if result.RowsAffected==0{return c.SendStatus(404)};return c.Type("html").SendString("<p>"+html.EscapeString("You have been unsubscribed from marketing emails.")+"</p>")})
}
''')
p=api/'db/db.go';s=p.read_text(encoding='utf-8').replace('&models.PartnerStore{},','&models.PartnerStore{},\n &models.EmailCampaign{},&models.EmailDelivery{},&models.EmailPreference{},&models.EmailAutomation{},',1);put('db/db.go',s)
p=api/'routes/routes.go';s=p.read_text(encoding='utf-8').replace('RegisterCommerceGrowth(api, auth, deps)','RegisterEmailCampaigns(api, auth, deps)\n RegisterCommerceGrowth(api, auth, deps)',1);put('routes/routes.go',s)
p=api/'main.go';s=p.read_text(encoding='utf-8').replace('routes.Register(app, deps)','routes.Register(app, deps)\n go services.RunEmailAutomation(database,cfg.BaseURL)',1);put('main.go',s)
print('Added scheduled email campaigns, unsubscribe preferences, delivery tracking and idempotent background automation (disabled until admin enables).')
