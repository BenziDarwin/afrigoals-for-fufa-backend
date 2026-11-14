from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def put(root,name,text):
 path=root/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_text(text,encoding='utf-8')
p=web/'app/(admin)/admin/partner-stores/page.tsx';s=p.read_text(encoding='utf-8')
s=s.replace('their real contact links','their contact numbers and social account names')
for old,new in [('Website URL','Website name'),('Instagram URL','Instagram name'),('TikTok URL','TikTok name'),('WhatsApp URL','WhatsApp number')]:s=s.replace(old,new)
start=s.index('                type={');end=s.index('                required=',start)
s=s[:start]+'''                type={key === "phone" || key === "whatsapp" ? "tel" : "text"}
                inputMode={key === "phone" || key === "whatsapp" ? "tel" : "text"}
                placeholder={key === "phone" || key === "whatsapp" ? "+256708021202" : key === "instagram" || key === "tiktok" ? "Vargo Sports or @vargosports" : undefined}
'''+s[end:]
put(web,'app/(admin)/admin/partner-stores/page.tsx',s)
p=web/'app/(app)/partner-stores/page.tsx';s=p.read_text(encoding='utf-8')
start=s.index('            <div className="flex flex-wrap gap-4">');end=s.index('            </div>',start)+len('            </div>')
s=s[:start]+'''            <dl className="space-y-2 text-sm">
              {([
                ["Website name", store.website],
                ["Instagram", store.instagram],
                ["TikTok", store.tiktok],
                ["WhatsApp", store.whatsapp],
              ] as const).filter(([, value]) => value).map(([label, value]) => (
                <div key={label} className="flex flex-wrap gap-x-2">
                  <dt className="font-medium">{label}:</dt><dd>{value}</dd>
                </div>
              ))}
            </dl>'''+s[end:]
put(web,'app/(app)/partner-stores/page.tsx',s)
put(api,'services/partner_contacts.go','''package services

import (
 "errors"
 "net/url"
 "regexp"
 "strings"
 "afrigoals-api.com/models"
)

var partnerPhonePattern = regexp.MustCompile(`^\\+?[0-9]{7,15}$`)

func contactName(value string) string {
 value = strings.TrimSpace(value)
 // Preserve existing records, displaying the name rather than a stored URL.
 if u,err:=url.Parse(value);err==nil&&(u.Scheme=="https"||u.Scheme=="http")&&u.Host!="" {
  if path:=strings.Trim(u.Path,"/");path!="" { parts:=strings.Split(path,"/");return strings.TrimPrefix(parts[len(parts)-1],"@") }
  return u.Hostname()
 }
 return value
}

func contactNumber(value string) string {
 value = strings.TrimSpace(value)
 if u,err:=url.Parse(value);err==nil&&(u.Scheme=="https"||u.Scheme=="http") {
  switch strings.ToLower(u.Hostname()) {
  case "wa.me": value=strings.Trim(u.Path,"/")
  case "api.whatsapp.com", "web.whatsapp.com": value=u.Query().Get("phone")
  }
 }
 return strings.NewReplacer(" ","","-","","(","",")","",".","").Replace(value)
}

func NormalizePartnerContacts(store *models.PartnerStore) error {
 store.Name=strings.TrimSpace(store.Name)
 store.Website=contactName(store.Website)
 store.Instagram=contactName(store.Instagram)
 store.TikTok=contactName(store.TikTok)
 store.Phone=contactNumber(store.Phone)
 store.WhatsApp=contactNumber(store.WhatsApp)
 for _,name:=range []string{store.Website,store.Instagram,store.TikTok} {
  if len(name)>150{return errors.New("website and social account names must be at most 150 characters")}
 }
 if store.Phone!=""&&!partnerPhonePattern.MatchString(store.Phone){return errors.New("enter a valid contact phone number (7 to 15 digits)")}
 if store.WhatsApp!=""&&!partnerPhonePattern.MatchString(store.WhatsApp){return errors.New("WhatsApp must be a phone number (7 to 15 digits), not an account name")}
 return nil
}
''')
p=api/'routes/commerce_growth.go';s=p.read_text(encoding='utf-8').replace('\n\t"net/url"','')
start=s.index('func validStoreURL(');end=s.index('func RegisterCommerceGrowth(',start);s=s[:start]+s[end:]
start=s.index('\t\tfor _, link := range []string{store.Website');end=s.index('\n\t\tif err := deps.DB.Save(&store)',start)
s=s[:start]+'''        if err:=services.NormalizePartnerContacts(&store);err!=nil {
            return c.Status(400).JSON(fiber.Map{"error":err.Error()})
        }'''+s[end:]
s=s.replace('return c.JSON(stores)','for i:=range stores{_ = services.NormalizePartnerContacts(&stores[i])}\n return c.JSON(stores)')
s=s.replace('evidence := fiber.Map{','for i:=range stores{_ = services.NormalizePartnerContacts(&stores[i])}\n evidence := fiber.Map{',1)
s=s.replace('Share only the listed phone and social/website links.', 'Share only the listed phone/WhatsApp numbers and website/social account names. Do not create, guess or show partner URLs from account names.')
put(api,'routes/commerce_growth.go',s)
p=api/'routes/commerce_growth_test.go';s=p.read_text(encoding='utf-8');start=s.index('func TestPartnerStoreLinksRejectExecutableSchemes(');s=s[:start];put(api,'routes/commerce_growth_test.go',s)
put(api,'services/partner_contacts_test.go','''package services
import("testing";"afrigoals-api.com/models")
func TestPartnerContactsAcceptNamesAndNumbers(t *testing.T){
 store:=models.PartnerStore{Name:"Vargo Sports",Website:"vargo sports",Instagram:"vargo sports",TikTok:"@vargosports",Phone:"+256 708 021202",WhatsApp:"+256708021202"}
 if err:=NormalizePartnerContacts(&store);err!=nil{t.Fatal(err)}
 if store.Instagram!="vargo sports"||store.WhatsApp!="+256708021202"{t.Fatal("contact names or numbers changed unexpectedly")}
}
func TestPartnerWhatsAppRequiresANumber(t *testing.T){
 store:=models.PartnerStore{Name:"Vargo",WhatsApp:"vargo sports"}
 if NormalizePartnerContacts(&store)==nil{t.Fatal("WhatsApp account name accepted instead of a number")}
}
func TestLegacyPartnerURLsDisplayAsNamesAndNumbers(t *testing.T){
 store:=models.PartnerStore{Instagram:"https://www.instagram.com/vargosports/",TikTok:"https://www.tiktok.com/@vargosports",WhatsApp:"https://wa.me/256708021202"}
 if err:=NormalizePartnerContacts(&store);err!=nil{t.Fatal(err)}
 if store.Instagram!="vargosports"||store.TikTok!="vargosports"||store.WhatsApp!="256708021202"{t.Fatalf("legacy contacts not converted: %#v",store)}
}
''')
print('Changed partner contacts to account names and telephone numbers; preserved existing records without a schema migration.')
