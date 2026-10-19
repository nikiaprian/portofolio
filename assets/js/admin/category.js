(function () {
  var NS = window.AdminNameSlugCrud;
  if (!NS) return;

  NS.bind({
    apiBase: "/api/v1/admin/categories",
    listPrefix: "categories",
    emptyIcon: "bi-folder2",
    ids: {
      createModal: "modalCategoryCreate",
      formCreate: "form-category-create",
      createError: "category-create-error",
      createParent: "cc-parent",
      createName: "cc-name",
      createSlug: "cc-slug",
      createTitle: "cc-title",
      createKeyword: "cc-keyword",
      createDescription: "cc-description",
      createEnabled: "cc-enabled",
      editModal: "modalCategoryEdit",
      formEdit: "form-category-edit",
      editError: "category-edit-error",
      editId: "ce-id",
      editParent: "ce-parent",
      editName: "ce-name",
      editSlug: "ce-slug",
      editTitle: "ce-title",
      editKeyword: "ce-keyword",
      editDescription: "ce-description",
      editEnabled: "ce-enabled",
      deleteModal: "modalCategoryDelete",
      deleteError: "category-delete-error",
      deleteConfirm: "cd-confirm",
      deleteDisplayId: "cd-id",
      deleteDisplayName: "cd-name",
      deleteDisplaySlug: "cd-slug",
      deleteDisplayTitle: "cd-title",
      deleteDisplayKeyword: "cd-keyword",
      deleteDisplayDescription: "cd-description",
    },
    messages: {
      loadFailed: "Gagal memuat kategori",
      noItems: "Kategori tidak ditemukan",
      emptyPage: "Tidak ada kategori di halaman ini.",
      createSuccess: "Kategori berhasil dibuat.",
      updateSuccess: "Kategori berhasil diperbarui.",
      deleteSuccess: "Kategori berhasil dihapus.",
      createFailed: "Gagal membuat kategori",
      updateFailed: "Gagal memperbarui kategori",
      deleteFailed: "Gagal menghapus kategori",
      loadOneFailed: "Gagal memuat kategori",
      bulkDeleteTitle: "Hapus kategori yang dipilih?",
      bulkDeleteText: function (count) {
        return "Tindakan ini akan menghapus permanen " + count + " kategori yang dipilih.";
      },
      bulkDeleteSuccess: function (deleted, failed) {
        if (failed > 0) {
          return deleted + " kategori terhapus, " + failed + " gagal.";
        }
        return deleted + " kategori berhasil dihapus.";
      },
      network: "Terjadi gangguan jaringan. Coba lagi.",
      pageOf: function (a, b) {
        return "Halaman " + a + " dari " + b;
      },
      dragReorderTitle: "Geser untuk mengatur ulang",
      orderUpdated: "Urutan kategori berhasil diperbarui.",
    },
    publicHrefForItem: function (item) {
      if (!item) return "";
      var slug = item.slug ? String(item.slug).trim() : "";
      if (!slug) return "";
      var AC = window.AdminCrud;
      var pslug =
        item.parent && item.parent.slug
          ? String(item.parent.slug).trim()
          : "";
      if (AC && typeof AC.publicCategoryHref === "function") {
        return AC.publicCategoryHref(slug, pslug);
      }
      if (AC && typeof AC.publicSitePath === "function") {
        var path = "/" + encodeURIComponent(slug);
        if (pslug) {
          path =
            "/" + encodeURIComponent(pslug) + "/" + encodeURIComponent(slug);
        }
        return AC.publicSitePath(path);
      }
      var fallback = "/" + encodeURIComponent(slug);
      if (pslug) {
        fallback =
          "/" + encodeURIComponent(pslug) + "/" + encodeURIComponent(slug);
      }
      return fallback;
    },
    enableDescription: true,
    enableCategorySort: false,
    enableCategoryDragSort: true,
    enableCategorySEO: true,
    enableCategoryTitle: true,
    enableCategoryParent: true,
  });
})();
