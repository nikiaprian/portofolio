package seeds

// Default bulk-post prompt text for fresh DB seeds. Kept in sync with
// internal/routes/handler/post_handler.go fallbacks (bulkTitlesPromptPrefix,
// defaultBulkPostPromptTemplate, meta hint defaults, and image prompt defaults).

const defaultSeedBulkPromptTitle = `Sarankan judul artikel blog yang singkat. Balas dengan teks biasa saja: satu judul per baris, tanpa penomoran, tanpa poin, tanpa kutipan, tanpa komentar tambahan..

`

const defaultSeedBulkPromptTitleEN = `Suggest short blog article titles. Reply with plain text only: one title per line, no numbering, no bullet points, no quotes, and no extra commentary.

`

const defaultSeedBulkPostPromptTemplate = `Buatkan artikel lengkap dan teroptimasi SEO tentang {{title}} yang unik, informatif, dan mudah dipahami. Artikel harus memenuhi kriteria berikut:
1. Panjang: Minimal 1000 kata.
2. Struktur: Harus memiliki struktur yang jelas dengan penggunaan subjudul yang efektif (H1, H2, H3) untuk memudahkan pembacaan dan meningkatkan SEO. Setiap bagian harus secara langsung menangani aspek yang berbeda dari {{title}} sesuai dengan User Intent.
Kamu bisa menempatkan keyword pada tengah parapraph pertama atau pada bagian akhir paragraph pertama.
Jadikan artikel ini agar masuk google snippet dan sekaligus discover. Untuk setiap subheading wajib memiliki satu paragraph pembuka, misalkan ada bullet, kamu bisa buat dulu satu paragraph pembuka lalu masuk pada point tersebut.
Jadikan artikel ini beda dengan yang sudah ada di internet, kamu bisa memulai subheading dengan menjelaskan topik atau tema artikel, masuk pada solusi dan terakhir paragraph penutup.
Agar artikel tidak terkesan sama dari sumber yang ada di internet kamu dapat memasukan subheading lainnya, seperti perbandingan, atau jika ada artikel listicle kamu bisa jelaskan satu persatu.
Gunakan Bahasa sederhana, kamu bisa pakai subject atau tidak. Kalau pakai subject dapat gunakan kalian atau kamu jangan anda.
Untuk penggunaan table hanya opsional, jika dibutuhkan bisa ditambahkan.
Untuk paragraph pembuka minimal 3 paragraph, dan jangan ada kalimat seperti pada artikel ini, topik yang banyak dicari, pada pembahasan ini, Kamu buat senatural mungkin.
Prioritas utama adalah panjang artikel. Jangan berhenti menulis sebelum minimal 2500 kata tercapai, walaupun struktur sudah terlihat lengkap. Perluas setiap subheading dengan penjelasan, contoh, perbandingan, dan elaborasi tambaha
Untuk FAQ bisa ditaruh setelah paragraph penutup untuk FAQ sendiri subheading 2 dan pertanyaan subheading 3.
Setiap paragraph jangan terlalu panjang dan jangan ada icon pada artikel, selain itu jangan ada garis untuk setiap subheading.
Untuk keyword turunan kamu dapat mencari di hasil pencarian google di bawah kolom, ambil 3 atau 4 keyword saja.
Jika ada cara atau tutorial maka kamu bisa gunakan bullet agar masuk snippet, selain itu apabila terdapat seperti kata portal resmi, maka kamu bisa tambahkan linknya, jadi lebih spesifik.
Jangan gunakan kata alur, karena ini menurut saya sangat kaku, kamu dapat gunakan cara atau lainnya.
Jangan pakai <h1> — judul artikel sudah di kolom title CMS. Mulai dengan satu atau lebih paragraf <p> pembuka, lalu isi memakai <h2>, <h3>, <h4>, <ul>, <ol>, <li>, <strong>, <u>, <em>, <blockquote>, <table> bila perlu.

3. Kesimpulan: Berikan kesimpulan atau penutup yang kuat dan informatif di akhir artikel yang merangkum poin utama dan memperkuat nilai yang ditawarkan kepada pembaca terkait {{title}}.
4. FAQ: Sertakan bagian FAQ di akhir yang menjawab pertanyaan umum terkait {{title}}. Ini menambah nilai bagi pembaca dan meningkatkan SEO dengan mengakomodasi long-tail keyword queries.

Pastikan {{content}}:

- Unik: Tidak ada duplikasi {{content}} dari internet.
- Relevan: Semua informasi harus relevan dengan {{title}} dan mencerminkan User Intent.
- Mudah Dipahami: Gunakan bahasa yang sederhana dan jelas.
- Terkini: Informasi harus sesuai dengan tren dan data terkini.
- Tidak terdeteksi sebagai artikel hasil AI.

Fokuskan artikel pada memberikan nilai dan informasi yang berguna bagi pembaca yang tertarik pada {{title}}. Gunakan penelitian dan data yang relevan untuk mendukung poin dan klaim dalam artikel. Jelaskan bagaimana {{title}} relevan dengan kebutuhan atau masalah spesifik yang dihadapi oleh pembaca, dan bagaimana artikel Anda menawarkan solusi atau wawasan.

Catatan: Tolong pastikan bahwa hasilnya unik dan belum pernah diterbitkan di tempat lain di internet.
Jangan gunakan subjeck pada artikel seperti anda, kamu dan sebagainya. Ini salah satu sumbernya (masukan artikel sumber)
`

const defaultSeedBulkPromptMetaDescription = `Ringkas meta description (sekitar 150-160 karakter) yang relevan dengan "{{title}}".`

const defaultSeedBulkPromptMetaKeywords = `Gunakan kata kunci utama secara natural yang relevan dengan "{{title}}"; hindari keyword stuffing. Jika perlu, sebutkan sinonim atau istilah terkait yang relevan bagi pembaca.`

const defaultSeedBulkPostPromptTemplateEN = `Write a complete SEO-optimized blog article about {{title}} that is unique, informative, and easy to understand. Requirements:
1. Length: at least 1000 words (longer is better when helpful).
2. Structure: start with one or more opening <p> paragraphs, then use <h2>, <h3>, <h4>, <ul>, <ol>, <li>, <strong>, <em>, <blockquote>, and <table> only when needed. Do NOT use <h1> (the CMS already has the title).
3. Make it scannable and snippet-friendly: each subheading should have an intro paragraph before bullets/lists.
4. Conclusion: end with a strong summary that reinforces the key takeaways.
5. FAQ: add an FAQ section at the end with common questions related to {{title}} (H2 for FAQ, H3 for each question).

Quality rules:
- Original: do not copy from the internet; do not mention that you are an AI.
- Relevant: everything must support the reader’s intent for {{title}}.
- Clear: use simple English and short paragraphs.
- Up-to-date: use current best practices and realistic details; avoid made-up statistics.

Return HTML only (no markdown fences).`

const defaultSeedBulkPromptMetaDescriptionEN = `Write a concise meta description (around 150–160 characters) relevant to "{{title}}".`

const defaultSeedBulkPromptMetaKeywordsEN = `Provide natural, relevant keywords for "{{title}}"; avoid keyword stuffing. Include a few related terms if helpful.`

const defaultSeedGenerateImagePrompt = `Buat gambar cover artikel berkualitas sedang dengan rasio horizontal (landscape) yang umum untuk artikel, utamakan 3:2 (setara 1024×683 px), berdasarkan judul: "{{title}}".

Patokan utama:
- Seluruh konsep visual WAJIB mengacu pada makna judul artikel/post.
- Pilih satu subjek visual utama yang paling mewakili judul.
- Jika judul spesifik (orang, profesi, tempat, peristiwa, produk), tampilkan elemen yang relevan dan mudah dipahami.

Gaya dan kualitas:
- Gaya editorial modern untuk berita/blog.
- Komposisi bersih, fokus jelas, pencahayaan realistis/sinematik.
- Kontras kuat, detail tajam, cocok untuk cover artikel/thumbnail website.
- Subjek utama ditempatkan aman di area tengah agar tetap bagus saat di-crop.

Batasan:
- Tanpa teks, logo, watermark, elemen UI, atau brand mark.
- Hindari blur berlebihan, noise, anatomi rusak, dan latar terlalu ramai.
`

const defaultSeedGenerateImageToContentPrompt = `Buat ilustrasi artikel berkualitas sedang untuk disisipkan di tengah isi tulisan (bukan thumbnail cover), rasio landscape atau netral (utamakan 3:2 atau 16:9), berdasarkan judul: "{{title}}".

Patokan utama:
- Visual mendukung atau menjelaskan gagasan utama topik artikel; cocok dibaca bersama paragraf di sekitarnya.
- Hindari komposisi yang hanya optimal untuk crop kecil seperti thumbnail feed.

Gaya dan kualitas:
- Gaya editorial modern; fokus jelas, pencahayaan realistis/sinematik.
- Satu ide visual utama; latar tidak mengganggu.

Batasan:
- Tanpa teks, logo, watermark, elemen UI, atau brand mark.
- Hindari blur berlebihan, noise, anatomi rusak, dan latar terlalu ramai.
`

const defaultSeedGenerateImagePromptEN = `Create a medium-quality cover image for a blog article in a landscape aspect ratio, preferably 3:2 (about 1024×683 px), based on the title: "{{title}}".

Guidelines:
- The visual concept must match the meaning of the title.
- Choose one main subject that best represents the title.
- Modern editorial/news/blog style, clean composition, realistic/cinematic lighting.

Constraints:
- No text, no logos, no watermarks, no UI elements, no brand marks.
- Avoid excessive blur/noise or distorted anatomy.`

const defaultSeedGenerateImageToContentPromptEN = `Create a medium-quality in-article illustration (not the cover thumbnail), landscape or neutral aspect ratio (prefer 3:2 or 16:9), based on the title: "{{title}}".

Guidelines:
- Support the main idea of the article and fit well between paragraphs.
- Modern editorial style, clean focus, uncluttered background.

Constraints:
- No text, no logos, no watermarks, no UI elements, no brand marks.
- Avoid excessive blur/noise or distorted anatomy.`
