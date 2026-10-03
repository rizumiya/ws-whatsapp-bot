# 📖 Panduan API WhatsApp (Untuk Pemula)

Selamat datang di dokumentasi API WhatsApp Gateway! Tutorial ini dibuat khusus agar orang awam sekalipun dapat memahami cara menggunakan sistem ini dari nol.

---

## 🌟 Konsep Dasar (Wajib Dibaca)

Sistem ini bersifat **Multi-Session**. Artinya, Anda bisa menyambungkan banyak nomor WhatsApp sekaligus ke dalam satu server.
Setiap nomor WhatsApp yang disambungkan disebut **Sesi (Session)**.

Untuk alasan keamanan, sistem ini menggunakan 2 jenis "Kunci Masuk" (API Key):
1. **Master API Key**: Kunci rahasia pusat (dari file `.env`). Gunanya HANYA untuk membuat sesi baru atau menghapus sesi.
2. **Session API Key**: Kunci khusus yang dihasilkan saat Anda membuat sesi baru. Kunci ini dipakai untuk **mengirim pesan** menggunakan nomor tersebut.

Semua API Key harus dikirimkan di bagian **Headers** HTTP dengan format:
`x-api-key: KUNCI_ANDA`

---

## 🚀 Alur Penggunaan (Dari Nol Sampai Kirim Pesan)

### Langkah 1: Mendaftarkan Akun / Sesi Baru
Anda harus mendaftarkan nomor WhatsApp Anda ke server agar mendapatkan *Session API Key*.

- **Endpoint**: `POST /api/v1/sessions`
- **Headers**: `x-api-key: (Isi dengan MASTER_API_KEY dari file .env)`
- **Payload (Body) JSON**:
```json
{
  "sessionId": "nomor-cs-1",
  "webhookUrl": "https://domain-anda.com/webhook" 
}
```
*(Catatan: `webhookUrl` opsional, berguna jika Anda ingin menerima notifikasi saat ada pesan masuk).*

- **Response Berhasil**:
```json
{
  "success": true,
  "data": {
    "sessionId": "nomor-cs-1",
    "apiKey": "a1b2c3d4e5f6g7h8i9j0...", 
    "status": "STARTING"
  }
}
```
> [!IMPORTANT]
> **SIMPAN `apiKey` INI BAIK-BAIK!** Kunci ini hanya ditampilkan sekali. Anda akan menggunakannya untuk mengirim pesan di langkah selanjutnya.

### Langkah 2: Mengambil QR Code untuk Login
Setelah sesi dibuat, server sedang mencoba terhubung. Anda perlu men-scan QR Code dari aplikasi WhatsApp di HP Anda (Pilih *Linked Devices* / Perangkat Taut).

- **Endpoint**: `GET /api/v1/sessions/nomor-cs-1/qr`
- **Headers**: `x-api-key: (Isi dengan Session API Key dari Langkah 1)`

- **Response**:
```json
{
  "success": true,
  "data": {
    "qr": "2@xyz1234567890..."
  }
}
```
*Gunakan text QR ini dan ubah menjadi gambar QR Code di website/aplikasi Anda menggunakan library seperti `qrcode`.*

> [!TIP]
> Jika Anda malas melakukan Scan QR, Anda bisa meminta **Pairing Code** (Kode 8 digit) untuk login tanpa kamera. Gunakan endpoint:
> `POST /api/v1/sessions/nomor-cs-1/pairing-code`
> Body: `{ "phoneNumber": "62812345678" }`

### Langkah 3: Mengirim Pesan Teks
Setelah berhasil login (Scan QR selesai), Anda bisa mulai mengirim pesan!

- **Endpoint**: `POST /api/v1/sessions/nomor-cs-1/messages/text`
- **Headers**: `x-api-key: (Isi dengan Session API Key dari Langkah 1)`
- **Payload (Body) JSON**:
```json
{
  "to": "628123456789@s.whatsapp.net",
  "text": "Halo, ini pesan otomatis dari bot!"
}
```
> [!NOTE]
> Format tujuan (`to`) HARUS diakhiri dengan `@s.whatsapp.net` untuk chat pribadi, atau `@g.us` untuk grup. Angka depan harus menggunakan kode negara (62) tanpa simbol `+`.

### Langkah 4: Mengirim Media (Gambar/Video/Dokumen)
- **Endpoint**: `POST /api/v1/sessions/nomor-cs-1/messages/media`
- **Headers**: `x-api-key: (Isi dengan Session API Key dari Langkah 1)`
- **Payload (Body) JSON**:
```json
{
  "to": "628123456789@s.whatsapp.net",
  "type": "image", 
  "url": "https://contoh.com/gambar.jpg",
  "caption": "Ini gambar keren!"
}
```
*(Tipe yang didukung: `image`, `video`, `audio`, `document`, `sticker`)*

> [!NOTE]
> Selain `url`, file bisa dikirim sebagai `base64` (boleh juga data URL `data:image/png;base64,...`). Ukuran permintaan dibatasi `MEDIA_BODY_LIMIT` (default `25mb`, kira-kira file 18 MB). File yang lebih besar kirim lewat `url`; permintaan yang melebihi batas dijawab `413 PAYLOAD_TOO_LARGE`.

---

## 🧰 Referensi Endpoint Lainnya

Selain fungsi dasar di atas, API ini mendukung fitur interaktif layaknya Anda menggunakan HP sendiri:

1. **Memberi Reaksi (Emoji)**
   `POST /api/v1/sessions/:sessionId/messages/react`
   Body: `{ "to": "...", "messageId": "ID_PESAN", "emoji": "❤️" }`

2. **Menarik Pesan (Unsend / Revoke)**
   `POST /api/v1/sessions/:sessionId/messages/revoke`
   Body: `{ "to": "...", "messageId": "ID_PESAN" }`

3. **Membaca Pesan (Centang Biru)**
   `POST /api/v1/sessions/:sessionId/chats/read`
   Body: `{ "to": "...", "messageId": "ID_PESAN" }`

4. **Status Typing (Sedang Mengetik...)**
   `POST /api/v1/sessions/:sessionId/chats/presence`
   Body: `{ "to": "...", "presence": "composing" }`

> [!TIP]
> File spesifikasi OpenAPI utuh (`openapi.yaml`) sudah saya buatkan di dalam direktori project Anda. Anda bisa mengunggah file tersebut ke **Postman** atau **Swagger Editor** (editor.swagger.io) untuk melihat seluruh API secara visual dan interaktif!
