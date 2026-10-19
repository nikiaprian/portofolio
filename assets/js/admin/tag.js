(function () {
  var NS = window.AdminNameSlugCrud;
  if (!NS) return;

  NS.bind({
    apiBase: "/api/v1/admin/tags",
    listPrefix: "tags",
    emptyIcon: "bi-tags",
    ids: {
      createModal: "modalTagCreate",
      formCreate: "form-tag-create",
      createError: "tag-create-error",
      createName: "tgc-name",
      createSlug: "tgc-slug",
      editModal: "modalTagEdit",
      formEdit: "form-tag-edit",
      editError: "tag-edit-error",
      editId: "tge-id",
      editName: "tge-name",
      editSlug: "tge-slug",
      deleteModal: "modalTagDelete",
      deleteError: "tag-delete-error",
      deleteConfirm: "tgd-confirm",
      deleteDisplayId: "tgd-id",
      deleteDisplayName: "tgd-name",
      deleteDisplaySlug: "tgd-slug",
    },
    messages: {
      loadFailed: "Gagal memuat tag",
      noItems: "Tag tidak ditemukan",
      emptyPage: "Tidak ada tag di halaman ini.",
      createSuccess: "Tag berhasil dibuat.",
      updateSuccess: "Tag berhasil diperbarui.",
      deleteSuccess: "Tag berhasil dihapus.",
      createFailed: "Gagal membuat tag",
      updateFailed: "Gagal memperbarui tag",
      deleteFailed: "Gagal menghapus tag",
      loadOneFailed: "Gagal memuat tag",
      bulkDeleteTitle: "Hapus tag yang dipilih?",
      bulkDeleteText: function (count) {
        return "Tindakan ini akan menghapus permanen " + count + " tag yang dipilih.";
      },
      bulkDeleteSuccess: function (deleted, failed) {
        if (failed > 0) {
          return deleted + " tag terhapus, " + failed + " gagal.";
        }
        return deleted + " tag berhasil dihapus.";
      },
      network: "Terjadi gangguan jaringan. Coba lagi.",
      pageOf: function (a, b) {
        return "Halaman " + a + " dari " + b;
      },
    },
    publicHrefForItem: function (item) {
      var slug = item && item.slug ? String(item.slug).trim() : "";
      if (!slug) return "";
      var AC = window.AdminCrud;
      if (AC && typeof AC.publicSitePath === "function") {
        return AC.publicSitePath("/tag/" + encodeURIComponent(slug));
      }
      return "/tag/" + encodeURIComponent(slug);
    },
  });
})();
