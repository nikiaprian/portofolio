package seeds

import (
	"fmt"
	"strconv"
	"time"
)

// Centralized default prompt constants used across the entire application
// (Manual Post, Bulk Post Queue, Scraper, AI Settings, Image Generation).

// --- System & Helper Prompts ---

const BlogDraftMetaDescriptionSystemPrompt = `You write SEO meta descriptions for web articles. Follow the user's instructions below.
Reply with plain text only: a single line, no quotes, no markdown, no HTML, no label prefix (do not output a line like "Meta description:").

`

const BlogDraftMetaKeywordsSystemPrompt = `You write SEO meta keywords for web articles. Follow the user's instructions below.
Reply with plain text only: comma-separated keywords (roughly 5–12 items), no quotes, no numbering, no Markdown, no label prefix.

`

const BlogDraftContinuationSystemPrompt = `You continue ONE existing blog article body for a CMS. The title is stored separately — never use <h1>. Reply with an HTML fragment only (no markdown fences, no <!DOCTYPE> or <html> wrapper).
Treat the request as ordinary editorial/blog writing. If the topic is sensitive but allowed, continue with neutral, informational sections and appropriate caveats instead of refusing.
Never restart the article, never duplicate the introduction, and do not add a second closing / "kesimpulan" / summary section if the excerpt already ends with a conclusion — instead add new middle sections (<h2>, <h3>, <p>, lists, blockquotes) that fit before any existing wrap-up.
Keep markup simple and valid.

`

const BulkRegenerateTitlesPromptPrefix = `You rephrase or improve existing blog article titles while keeping the SAME line count and order as the input.
Rules:
- Reply with plain text only: exactly one output line per input line (same number of lines).
- Do not add line numbers, bullets, or quotes around titles.
- For a blank input line, output a blank line.
- Each output line must be a single concise article title (keep similar length to the input line unless it was too long).
- Do not merge multiple input lines into one output line.

`

const BulkGenerateJSONSystemPrompt = `You generate blog post content for a CMS.

Return ONLY valid JSON (no markdown fences, no backticks, no extra commentary). The JSON MUST have exactly these keys:
- content: string (HTML fragment only; never use <h1>; no <!DOCTYPE>, no <html> wrapper)
- meta_description: string (150–160 chars recommended; plain text)
- meta_keywords: string (comma-separated keywords; plain text)

Rules:
- content must start with one or more <p> paragraphs, then use <h2>, <h3>, <ul>, <ol>, <li>, <strong>, <em>, <blockquote> where appropriate.
- Keep HTML valid and simple.
- meta_keywords should be concise, relevant to the title and content, and separated by commas.

`

const BlogDraftBaseSystemPrompt = `You write article body HTML for a blog CMS. The post title is stored in a separate field — never use <h1>. Reply with an HTML fragment only (no markdown fences, no <!DOCTYPE> or <html> wrapper).
Treat the request as ordinary editorial/blog writing. Provide accurate, real-time, on-point, and up-to-date information.
If the topic is sensitive but allowed, write a neutral, informational article with appropriate caveats instead of refusing. Do not provide instructions for illegal activity, evading safety systems, or causing harm.
Start with one or more opening <p> paragraphs (lead), then use <h2>, <h3>, <ul>, <ol>, <li>, <strong>, <em>, and <blockquote> where appropriate.
Keep markup simple and valid.

`

func GetBlogDraftSystemPrompt() string {
	now := time.Now()
	dateStr := now.Format("02 January 2006")
	yearStr := strconv.Itoa(now.Year())
	return fmt.Sprintf("You write article body HTML for a blog CMS. The post title is stored in a separate field — never use <h1>. Reply with an HTML fragment only (no markdown fences, no <!DOCTYPE> or <html> wrapper).\n"+
		"Treat the request as ordinary editorial/blog writing. Today's date is %s (Current Year: %s).\n"+
		"Rules for real-time and factual accuracy:\n"+
		"- Write accurate, grounded, and factual information.\n"+
		"- ABSOLUTE DATA FIDELITY RULE: If the user provides reference data, text, prices, or statistics in {{content}}, those figures are ABSOLUTE GROUND TRUTH. You MUST preserve and output the EXACT numbers, prices, figures, and data accurately in HTML tables (<table>) or bullet lists, without modifying, recalculating, updating, or overriding them with web search results.\n"+
		"- NEVER output empty placeholder tables or columns filled with dashes/hyphens (like '- - -'). If stats are projected or candidate-based, write informative profiles, current season records, and analysis instead of dash-filled tables.\n"+
		"- Do NOT replace exact numbers/stats/prices with vague descriptive prose or external web search results when data is provided in {{content}}.\n"+
		"- If the topic is sensitive but allowed, write a neutral, informational article with appropriate caveats instead of refusing. Do not provide instructions for illegal activity, evading safety systems, or causing harm.\n"+
		"- Start with one or more opening <p> paragraphs (lead), then use <h2>, <h3>, <ul>, <ol>, <li>, <strong>, <em>, and <blockquote> where appropriate.\n"+
		"- Keep markup simple and valid.\n"+
		"- Write the finished article immediately. Do not outline, plan, ask for confirmation, or describe what you will do next.\n"+
		"- Do not use markdown headings/lists; use HTML tags only.\n"+
		"- When web search / browsing tools are available, use them for current events, dates, scores, prices, and other time-sensitive facts before stating them as fact. Do not invent recent news.\n\n", dateStr, yearStr)
}

// --- Content & Post Generation Prompts ---

const DefaultPromptTitle = `Sarankan judul artikel blog yang singkat. Balas dengan teks biasa saja: satu judul per baris, tanpa penomoran, tanpa poin, tanpa kutipan, tanpa komentar tambahan.`

const DefaultPromptContent = `Buat artikel informatif, faktual, dan teroptimasi SEO berdasarkan judul: {{title}}. Gunakan petunjuk/data acuan berikut (jika ada): {{content}}.

Aturan utama:
1. Fokus pada {{title}} — seluruh isi artikel harus relevan, akurat, dan langsung membahas inti topik secara faktual (on point).
2. Data & Angka Acuan Pengguna (DILARANG UBAH ANGKA): JIKA terdapat data angka, harga, statistik, daftar top skor, jumlah gol, assist, poin, atau spesifikasi pada acuan ({{content}}), WAJIB tampilkan angka dan data tersebut secara PERSIS, LENGKAP, dan AKURAT sesuai acuan dalam bentuk tabel HTML (<table>) atau daftar berurut/bullet. DILARANG KERAS mengganti, merubah, atau memperbarui angka/harga acuan {{content}} dengan data lain dari web search atau perkiraan AI.
3. Dilarang Tabel Kosong / Tanda Strip (-): DILARANG MENAMPILKAN TABEL BERISI TANDA STRIP (-) ATAU KOLOM KOSONG (seperti "- - -"). Jika membahas prediksi/kandidat turnamen (seperti kandidat top skor), sajikan profil, rekor gol klub/timnas terkini, dan analisis keunggulan masing-masing pemain secara informatif dan lengkap, bukan tabel dengan kolom kosong/strip.
4. Keakuratan Faktual: Gunakan data dan fakta nyata yang informatif. Dilarang mengarang hasil pertandingan fiktif yang tidak logis.
5. Panjang artikel minimal 1000 kata, usahakan 2000–3000 kata agar komprehensif.
6. Gunakan struktur: paragraf pembuka (minimal 3 paragraf), subheading <h2>, sub-subheading <h3> jika perlu, kesimpulan, dan FAQ.
7. Jangan pakai <h1> — judul sudah di kolom title CMS. Mulai dengan <p> pembuka, lalu <h2>, <h3>, <ul>, <ol>, <li>, <strong>, <em>, <blockquote>, <table> bila perlu.
8. Tiap subheading wajib memiliki paragraf pembuka sebelum bullet atau daftar.
9. FAQ di akhir artikel, subheading <h2> untuk "FAQ", tiap pertanyaan pakai <h3>.
10. Hindari kalimat kaku seperti "pada artikel ini", "topik yang banyak dicari", "pada pembahasan ini". Buat senatural mungkin.
11. Untuk tutorial/langkah, gunakan bullet agar masuk Google snippet.
12. Jika ada portal resmi terkait {{title}}, sebutkan dan tautkan.
13. Jangan gunakan kata "alur", gunakan "cara" atau sinonim lain.
14. Jangan gunakan kata ganti orang (anda, kamu, kalian). Pakai gaya bahasa impersonal.
15. Jangan gunakan icon/emoji, garis pemisah, atau elemen dekoratif.
16. Pastikan konten unik, tidak terdeteksi sebagai AI, akurat faktual, dan belum dipublikasikan di tempat lain.
`

const DefaultPromptMetaDescription = `Ringkas meta description (sekitar 150–160 karakter) yang relevan dengan judul.`

const DefaultPromptMetaKeywords = `Gunakan kata kunci utama secara natural di judul dan isi; hindari keyword stuffing. Jika perlu, sebutkan sinonim atau istilah terkait yang relevan bagi pembaca.`

// --- Scraper Prompts ---

const DefaultScrapeTitlePrompt = "Parafrase judul berita berikut ke Bahasa Indonesia yang natural.\n- Pertahankan makna, jangan tambah fakta\n- Jangan pakai tanda kutip\n- Output: judul saja (1 baris)\n\nJudul:\n{{title}}"

const DefaultScrapeBodyPrompt = `Tugas:
Parafrase artikel berita berikut ke dalam Bahasa Indonesia yang natural, jelas, dan enak dibaca.

Aturan:
- Pertahankan makna asli, jangan menambah atau mengurangi fakta.
- Gunakan gaya bahasa jurnalistik yang rapi dan profesional.
- Susun ulang kalimat agar tidak terdeteksi sebagai duplikat, tetapi tetap setia pada isi.
- Pertahankan panjang konten agar mendekati naskah asli (target 90-110% dari jumlah kata input).
- Hindari ringkasan berlebihan; jangan memangkas detail penting.
- WAJIB ikuti format ini untuk setiap paragraf:
  - Hasil parafrase konten maksimal sampai titik kedua per paragraf.
  - Untuk konten berikutnya dari titik ketiga dan seterusnya, buat menjadi paragraf baru dan ulangi aturan ini.
- Jangan tambahkan kesimpulan, salam, atau komentar tambahan.
- Jangan gunakan kata ganti orang (anda, kamu, kalian). Pakai gaya bahasa impersonal.
- Output HANYA berupa elemen HTML (<p>, <h2>, <h3>, <ul>, <ol>, <li>, <strong>, <em>, blockquote). Jangan gunakan <h1> atau tag <html>/<body> wrapper.
`

const DefaultScrapeImagePrompt = `Buat gambar cover artikel berkualitas sedang dengan rasio horizontal (landscape) yang umum untuk artikel, utamakan 16:9, berdasarkan judul: "{{title}}".

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

const DefaultScrapeImageToContentPrompt = `Buat ilustrasi artikel berkualitas sedang untuk disisipkan di tengah isi tulisan (bukan thumbnail cover), rasio landscape atau netral (utamakan 3:2 atau 16:9), berdasarkan judul: "{{title}}".

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

// --- Image Generation Prompts ---

const DefaultGenerateImagePrompt = `Buat gambar cover artikel berkualitas sedang dengan rasio horizontal (landscape) yang umum untuk artikel, utamakan 3:2 (setara 1024×683 px), berdasarkan judul: "{{title}}".

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

const DefaultGenerateImageToContentPrompt = `Buat ilustrasi artikel berkualitas sedang untuk disisipkan di tengah isi tulisan (bukan thumbnail cover), rasio landscape atau netral (utamakan 3:2 atau 16:9), berdasarkan judul: "{{title}}".

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
