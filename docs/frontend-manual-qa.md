# Frontend Manual QA Checklist

Bu dokuman file manager arayuzu icin manuel test adimlarini listeler.

## 1) Login / Session
- Uygulamayi ac.
- Gecerli hesap ile giris yap.
- Beklenen: dashboard yuklenir, `Dosyalar` gorunur, hata flash'i yok.
- Gecersiz sifre ile tekrar dene.
- Beklenen: net hata mesaji gorunur, UI kilitlenmez.

## 2) Folder Navigation / Breadcrumb
- Root altinda bir klasore gir.
- Breadcrumb'dan onceki klasore geri don.
- Beklenen: liste dogru parent klasoru gosterir, secimler temizlenir.

## 3) Search + Type Filter
- Arama panelini ac, dosya adindan ara.
- Beklenen: sadece adla eslesen dosyalar listelenir.
- Tur filtresini `Gorsel`, `Metin`, `Klasor` vb. seceneklerle dene.
- Beklenen: secilen tipe uygun ogeler listelenir.
- Sonuc yoksa:
  - Beklenen: "Sonuc bulunamadi" bos durum karti ve "Arama/Filtre temizle" aksiyonu gorunur.

## 4) Upload (Picker + Drag & Drop)
- `+` menusu -> `Yukle` ile birden fazla dosya sec.
- Beklenen: bildirim panelinde dosya bazli ilerleme (%) gorunur.
- Workspace'e dosya surukle-birak.
- Beklenen: yukleme otomatik baslar, liste yenilenir.

## 5) Upload Reliability
- Yukleme devam ederken bildirim panelinden `Iptal et`.
- Beklenen: job durumu `CANCELLED` olur, detayda iptal nedeni gorunur.
- `Tekrar dene` ile ayni dosyayi yeniden yukle.
- Beklenen: upload yeniden baslar ve basariliysa `DONE`.
- Ag hatasi simule et (API'yi gecici durdur) ve yukleme dene.
- Beklenen: job `ERROR`, detayda hata nedeni, `Tekrar dene` aktif.

## 6) Rename
- Bir dosya sec -> 3 nokta -> `Yeniden adlandir`.
- Gecerli ad ile kaydet.
- Beklenen: liste yeni adla guncellenir.
- Bos ad dene.
- Beklenen: dogrulama/hata mesaji.

## 7) Move
- Tekli ve coklu secimle `Tasi`.
- Beklenen: hedef klasore tasinir, liste yenilenir.
- Klasoru kendi icine/alt klasorune tasimayi dene.
- Beklenen: islem engellenir, dogrulama mesaji gorunur.

## 8) Delete / Bulk Delete
- Tekli silme ve coklu silme dene.
- Beklenen: basarili silinenler listeden kalkar.
- Kismi hata durumunda (izin/ag sorunu):
  - Beklenen: kac adet basarili/hatali oldugu flash mesajinda acikca yazilir.

## 9) Preview
- Metin dosyasi sec.
- Beklenen: detay panelinde metin onizleme (snippet) gorunur, `Editor'de ac` calisir.
- Gorsel dosya sec (png/jpg/webp).
- Beklenen: detay panelinde image preview gorunur.
- Desteklenmeyen tur sec.
- Beklenen: fallback "Dosyayi indir" CTA gorunur.

## 10) Regression
- Liste ve grid modlari arasinda gecis yap.
- Beklenen: secim/context menu bozulmaz.
- Tema degisimi yap.
- Beklenen: kontrast ve okunabilirlik korunur.

