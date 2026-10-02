# به‌روزرسانی پنل از روی MWPanel18.zip

این دستورات رو **روی سرور، با SSH** اجرا کن (نه روی ویندوز خودت).

## 0) دو تا متغیر رو یک‌بار تنظیم کن

مسیر دقیق پوشه‌ی قدیمی پروژه و مسیر فایل zip که آپلود کردی رو اینجا بذار
(این‌ها فقط برای همین ترمینال هستن، دائمی نیستن):

```bash
OLD_PROJECT_DIR=/root/MWPanel-main      # مسیر واقعی پوشه‌ی قدیمی رو جایگزین کن
ZIP_PATH=/root/MWPanel18.zip            # مسیر واقعی فایل zip رو جایگزین کن
```

## 1) سرویس در حال اجرا رو متوقف کن

```bash
sudo systemctl stop mwp
```

> **نکته‌ی امنیتی مهم**: دیتابیس، فایل‌های پیر، و شناسه‌ی لایسنسِ سرور
> (`install-id`) همیشه داخل `$DATA_DIR` هستن (پیش‌فرض `~/.config/mwp`)،
> که کاملاً جدا از پوشه‌ی پروژه (`/root/mwp-deploy/...`) قرار داره. این
> مراحل هیچ‌وقت به `$DATA_DIR` دست نمی‌زنن، پس آپدیت باینری تأثیری روی
> لایسنس یا دیتای مشتری نداره. فقط مطمئن شو که پوشه‌ی پروژه‌ی قدیمی رو
> پاک نمی‌کنی طوری که به‌اشتباه `$DATA_DIR` هم توش باشه.

## 2) نسخه‌ی جدید رو باز کن (بدون پاک کردن نسخه‌ی قدیمی)

```bash
mkdir -p /root/mwp-deploy
cd /root/mwp-deploy
unzip -o "$ZIP_PATH" -d .
```

اگه zip یک پوشه‌ی داخلی مثل `MWPanel-main/` داره، بعد از unzip چک کن:

```bash
ls /root/mwp-deploy
```

و اگه دیدی یک پوشه‌ی تو در تو هست (مثلاً `/root/mwp-deploy/MWPanel-main/api`)،
از همون مسیر ادامه بده:

```bash
cd /root/mwp-deploy/MWPanel-main   # یا هر اسمی که واقعاً داخلشه
```

## 3) فرانت رو build بگیر -- **همیشه اول این کار**

این مرحله مهم‌ترین بخشه: باینری Go کل پوشه‌ی `ui/dist` رو مستقیم داخل خودش
کپی می‌کنه، پس اگه این مرحله رو رد کنی یا بعد از بک‌اند اجرا کنی، پنل همیشه
نسخه‌ی قدیمی فرانت رو نشون می‌ده -- هرچقدرم که سرویس رو ری‌استارت کنی.

```bash
cd ui
npm install
npm run build
cd ..
```

باید در پایان یک پوشه‌ی `ui/dist` پر از فایل ببینی:

```bash
ls ui/dist
```

## 4) بک‌اند رو build بگیر -- **بعد از فرانت**

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mwp -ldflags="-s -w" ./api/cmd
```

اگه سرور arm64 هست (نه x86_64)، `GOARCH=amd64` رو بذار `GOARCH=arm64`.
چک کردن معماری سرور:

```bash
uname -m
# x86_64  -> GOARCH=amd64
# aarch64 -> GOARCH=arm64
```

## 5) باینری جدید رو جایگزین کن

```bash
sudo install -v -o root -g root -m 755 mwp /usr/local/bin/mwp
```

## 6) سرویس رو دوباره راه‌انداز کن

```bash
sudo systemctl daemon-reload
sudo systemctl restart mwp
sudo systemctl status mwp --no-pager
```

باید ببینی `active (running)`.

## 7) تأیید کن که نسخه‌ی جدید واقعاً بالا اومده

```bash
curl -s http://127.0.0.1:3000/api/user-manager/protocol-config \
  -H "Authorization: Bearer <یک توکن معتبر>" | head -c 300
```

یا ساده‌تر: از مرورگر با یک Hard Refresh (Ctrl+Shift+R) پنل رو باز کن،
برو داخل User Manager و لینک ساب یک اکانت رو بگیر -- دیگه نباید 404 بده.
اگه هنوز 404 می‌ده، یعنی مرحله‌ی ۳ (build فرانت) قبل از مرحله‌ی ۴ اجرا نشده.

## اگه به مشکل خوردی

```bash
sudo journalctl -u mwp -n 100 --no-pager
```

خروجی رو برام بفرست.

---

### خلاصه‌ی کل فرآیند (فقط دستورات، بدون توضیح)

```bash
sudo systemctl stop mwp

mkdir -p /root/mwp-deploy && cd /root/mwp-deploy
unzip -o /root/MWPanel18.zip -d .
cd MWPanel-main   # اسم پوشه‌ی داخلی رو با ls چک کن

cd ui && npm install && npm run build && cd ..

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mwp -ldflags="-s -w" ./api/cmd

sudo install -v -o root -g root -m 755 mwp /usr/local/bin/mwp
sudo systemctl daemon-reload
sudo systemctl restart mwp
sudo systemctl status mwp --no-pager
```
