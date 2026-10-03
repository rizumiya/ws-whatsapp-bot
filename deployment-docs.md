# 🖥️ Panduan Menjalankan & Deploy Aplikasi

Dokumen ini berisi panduan tahap demi tahap untuk menjalankan server WhatsApp Gateway secara lokal di komputer Anda, hingga panduan mengunggahnya (Deploy) ke Server Hosting (VPS).

---

## 🏠 Bagian 1: Menjalankan di Komputer Lokal (Localhost)

Sebelum menjalankan aplikasi, pastikan komputer Anda sudah terinstal perangkat lunak berikut:
1. **Node.js** (Minimal versi 18)
2. **Docker Desktop** (Untuk menjalankan PostgreSQL dan Redis dengan mudah)
3. **Git** (Opsional)

### Langkah-langkah:
1. **Buka Terminal / Command Prompt** dan masuk ke folder project (`m:\ws-whatsapp-bot`).
2. **Copy file konfigurasi**:
   Jika belum ada, copy file `.env.example` menjadi `.env`.
   Buka file `.env` dan ganti `MASTER_API_KEY=your_secure_master_api_key` menjadi password rahasia Anda sendiri.
3. **Jalankan Database**:
   Ketik perintah berikut untuk mengunduh dan menyalakan database (PostgreSQL & Redis) di background:
   ```bash
   docker compose up -d
   ```
4. **Instal Modul / Dependencies**:
   Ketik perintah:
   ```bash
   npm install
   ```
5. **Jalankan Server Development**:
   Ketik perintah:
   ```bash
   npm run dev
   ```
   *Jika muncul tulisan `🚀 Server is running on port 3000` dan `✅ PostgreSQL connected`, selamat! Aplikasi Anda sudah menyala dan siap menerima request API di `http://localhost:3000`.*

---

## 🌍 Bagian 2: Panduan Deploy ke Hosting Production (VPS)

Aplikasi berbasis *headless-browser* / WebSocket seperti Baileys **TIDAK BISA** di-hosting di *Shared Hosting* biasa (seperti cPanel murah). Anda **WAJIB** menggunakan **VPS (Virtual Private Server)**.
Rekomendasi VPS: DigitalOcean, Linode, AWS EC2, atau VPS lokal ber-OS **Ubuntu 20.04 / 22.04**.

### Apa Saja yang Perlu Dipersiapkan di VPS?
1. **OS**: Ubuntu Linux
2. **Node.js** v18+ & **NPM**
3. **PM2**: Aplikasi manajer untuk menjaga bot tetap hidup 24/7 meskipun Anda menutup terminal VPS.
4. **PostgreSQL & Redis**: Anda bisa menginstalnya langsung di VPS atau menggunakan Docker.
5. **Nginx** (Opsional): Digunakan sebagai Reverse Proxy agar API Anda memiliki domain (contoh: `api.domain.com`) dan bersertifikat SSL (HTTPS).

### Langkah Deploy ke Ubuntu VPS:

**1. Clone atau Upload Kode ke VPS**
Bawa file project Anda ke VPS (bisa pakai Github `git clone` atau FTP). Abaikan folder `node_modules` dan `.env`.

**2. Setup Environment**
Masuk ke folder project di dalam VPS, lalu buat file `.env` baru:
```bash
nano .env
```
*(Isi `.env` seperti di localhost, tapi pastikan `NODE_ENV=production` dan `LOG_LEVEL=info`)*

**3. Install Dependencies dan Build**
```bash
npm install
npm run build
```
*(Perintah build akan mengubah kode TypeScript di folder `src` menjadi kode JavaScript di folder `dist`)*

**4. Install PM2 (Secara Global)**
```bash
npm install -g pm2
```

**5. Jalankan Aplikasi dengan PM2**
Karena ini production, kita jalankan file `server.js` hasil build di folder `dist`:
```bash
pm2 start dist/server.js --name "whatsapp-bot"
```

**6. Pastikan Bot Menyala Saat Server Restart**
Agar jika VPS di-restart bot otomatis menyala kembali, jalankan:
```bash
pm2 startup
pm2 save
```

> [!TIP]
> **Cara Melihat Log di VPS:**
> Jika Anda ingin melihat pesan masuk atau error saat bot berjalan di VPS, ketik:
> `pm2 logs whatsapp-bot`

> [!IMPORTANT]
> **Migrasi Database:**
> Ingat, tabel database PostgreSQL harus dibuat terlebih dahulu di VPS. Anda bisa meng-copy isi dari `database/migrations/*.sql` dan mengeksekusinya di PostgreSQL VPS Anda melalui DBeaver atau perintah `psql`.
