from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api'); web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def edit(root,name,old,new):
 p=root/name; s=p.read_text(encoding='utf-8')
 if name=='routes/auth.go':
  start=s.index('auth.Post("/reset-password/confirm"');end=s.index('auth.Post("/login"',start)
  block=s[start:end];assert old in block,(name,old[:70]);s=s[:start]+block.replace(old,new,1)+s[end:]
 else:
  assert old in s,(name,old[:70]);s=s.replace(old,new,1)
 p.write_text(s,encoding='utf-8')
def put(root,name,s):
 p=root/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(s,encoding='utf-8')

edit(api,'routes/riders.go','"afrigoals-api.com/dtos"','"afrigoals-api.com/dtos"\n "afrigoals-api.com/middleware"')
edit(api,'routes/riders.go','admin.Use(auth)','admin.Use(auth, middleware.RequireAdmin())')
edit(api,'services/jwt.go','\t})\n\n\tif err != nil {','\t}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(m.issuer), jwt.WithExpirationRequired())\n\n\tif err != nil {')
edit(api,'services/jwt.go','if !ok || !parsed.Valid {','if !ok || !parsed.Valid || claims.Subject == "" || claims.Email == "" || claims.Role == "" {')

p=api/'routes/orders.go';s=p.read_text(encoding='utf-8');start=s.index('\trouter.Get("/orders/:id",');end=s.index('\nfunc createOrderFromFlatDelivery',start);block=s[start:end]
block=block.replace('var order models.Order\n\t\tif err := deps.DB.', '''if userID == "" { return c.Status(401).JSON(fiber.Map{"error":"unauthorized"}) }
        query := deps.DB
        if !isAdminRole(role) { query = query.Where("user_id = ?", userID) }
        var order models.Order
        if err := query.''',1)
s=s[:start]+block+s[end:];p.write_text(s,encoding='utf-8')

# Atomically consume reset challenges; a concurrent request cannot reuse the code.
edit(api,'routes/auth.go','''\t\t\tchallenge.Attempts++
\t\t\t_ = deps.DB.Save(&challenge).Error

\t\t\treturn c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
\t\t\t\t"error": "invalid or expired code",''','''            _ = deps.DB.Model(&models.PasswordResetChallenge{}).Where("id = ? AND used_at IS NULL AND attempts < 5", challenge.ID).UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error

\t\t\treturn c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
\t\t\t\t"error": "invalid or expired code",''')
edit(api,'routes/auth.go','''\t\t\tif err := tx.Save(&challenge).Error; err != nil {
\t\t\t\treturn err
\t\t\t}
\t\t\treturn tx.Save(&user).Error''','''            consumed := tx.Model(&models.PasswordResetChallenge{}).
                Where("id = ? AND used_at IS NULL AND expires_at > ? AND attempts < 5", challenge.ID, now).
                Update("used_at", now)
            if consumed.Error != nil { return consumed.Error }
            if consumed.RowsAffected != 1 { return fiber.NewError(401, "invalid or expired code") }
            if err := tx.Model(&models.PasswordResetChallenge{}).Where("user_id = ? AND used_at IS NULL", user.ID).Update("used_at", now).Error; err != nil { return err }
            return tx.Model(&models.User{}).Where("id = ?", user.ID).Update("password_hash", hash).Error''')

# Do not let older profile responses overwrite the result of a newer sign-in.
edit(web,'lib/auth/client.ts','useCallback, useEffect, useState','useCallback, useEffect, useRef, useState')
edit(web,'lib/auth/client.ts','  const refresh = useCallback(async () => {','''  const requestVersion = useRef(0);
  const refresh = useCallback(async () => {
    const version = ++requestVersion.current;''')
edit(web,'lib/auth/client.ts','      if (!res.ok) {','      if (version !== requestVersion.current) return;\n      if (!res.ok) {')
edit(web,'lib/auth/client.ts','      const user = normalizeUser(data);','      if (version !== requestVersion.current) return;\n      const user = normalizeUser(data);')
edit(web,'lib/auth/client.ts','    } catch {\n      setState({','    } catch {\n      if (version !== requestVersion.current) return;\n      setState({')
edit(web,'lib/auth/client.ts','  const signOut = useCallback(async () => {','  const signOut = useCallback(async () => {\n    ++requestVersion.current;')

# Map listeners should use current callbacks, not the first render's selection.
edit(web,'components/checkout/DeliveryMapPicker.tsx','  const mapRef = useRef','  const onSelectRef = useRef(onSelect);\n  onSelectRef.current = onSelect;\n  const mapRef = useRef')
p=web/'components/checkout/DeliveryMapPicker.tsx';s=p.read_text(encoding='utf-8').replace('onSelect(location);','onSelectRef.current(location);').replace('onSelect(selectedLocation);','onSelectRef.current(selectedLocation);')
s=s.replace('let cancelled = false;','let cancelled = false;\n    setIsReady(false);',1).replace('  }, [isGoogleLoaded, initialCenter]);','  }, [isGoogleLoaded, initialCenter.lat, initialCenter.lng]);',1)
s=s.replace('''    return () => {
      cancelled = true;
    };
  }, [isGoogleLoaded''','''    return () => {
      cancelled = true;
      for (const instance of [autocompleteRef.current, markerRef.current, mapInstanceRef.current]) {
        if (instance) googleMaps.event?.clearInstanceListeners(instance);
      }
      if (typeof markerRef.current?.setMap === "function") markerRef.current.setMap(null);
      else if (markerRef.current) markerRef.current.map = null;
      autocompleteRef.current = null;
    };
  }, [isGoogleLoaded''')
s=s.replace('          ref={inputRef}\n', '          ref={inputRef}\n          disabled={!isReady || isResolving}\n',1);p.write_text(s,encoding='utf-8')

put(api,'routes/release_security_test.go','''package routes
import (
 "net/http/httptest"
 "testing"
 "github.com/gofiber/fiber/v2"
)
func TestCustomerCannotUseRiderAdministration(t *testing.T) {
 app:=fiber.New()
 RegisterRiderRoutes(app,func(c *fiber.Ctx)error{ c.Locals("userID","customer");c.Locals("role","user");return c.Next()},Deps{})
 for _,tt:=range []struct{method,path string}{{"GET","/admin/riders"},{"PATCH","/admin/riders/id/status"},{"POST","/admin/orders/id/assign-rider"}}{
  response,err:=app.Test(httptest.NewRequest(tt.method,tt.path,nil));if err!=nil{t.Fatal(err)};response.Body.Close()
  if response.StatusCode!=403{t.Fatalf("%s %s: %d",tt.method,tt.path,response.StatusCode)}
 }
}
''')
put(api,'services/release_security_test.go','''package services
import("math";"testing";"time")
func TestTrustedDeviceTokenCannotAuthorizeAPIRequests(t *testing.T){
 manager:=NewJWT(Config{JWTSecret:"test-only-secret-at-least-32-characters",JWTIssuer:"test"})
 trusted,err:=manager.GenerateTrustedDeviceToken("owner");if err!=nil{t.Fatal(err)}
 if _,err:=manager.Parse(trusted);err==nil{t.Fatal("trusted-device token used as bearer authentication")}
 access,err:=manager.GenerateToken("owner","owner@example.com","user",time.Hour);if err!=nil{t.Fatal(err)}
 if _,err:=manager.Parse(access);err!=nil{t.Fatal(err)}
 if manager.IsTrustedDevice(trusted,"other"){t.Fatal("trusted browser accepted for another account")}
}
func TestRiderPayoutReconcilesWithDeliveryFee(t *testing.T){
 t.Setenv("DELIVERY_SERVICE_FEE_RATE","0.15")
 for _,fee:=range []float64{0,300,5500,10000}{
 commission,payout:=calculateDeliveryPayout(fee)
 if math.Abs(commission+payout-fee)>.001{t.Fatalf("fee %v != %v+%v",fee,commission,payout)}
 if fee==300&&(commission!=45||payout!=255){t.Fatal("300 fee must pay rider 255 and Afrigoals 45")}
 }
}
''')
print('Applied commerce release authorization, password reset, profile refresh and map lifecycle fixes.')
