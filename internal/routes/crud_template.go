package routes

import "github.com/gin-gonic/gin"

// CrudListView builds the "crud" map expected by template admin-crud-list.
// prefix is used for element IDs: {prefix}-error, {prefix}-tbody, {prefix}-page-info, etc.
// createModalTarget can be "" to hide the create button (e.g. read-only list).
func CrudListView(prefix, cardTitle, createLabel, createIcon, createModalTarget string, columnHeaders []gin.H) gin.H {
	return gin.H{
		"error_id":              prefix + "-error",
		"page_info_id":          prefix + "-page-info",
		"page_size_id":          prefix + "-page-size",
		"prev_btn_id":           prefix + "-prev-btn",
		"next_btn_id":           prefix + "-next-btn",
		"tbody_id":              prefix + "-tbody",
		"search_input_id":       prefix + "-search",
		"card_title":            cardTitle,
		"create_label":          createLabel,
		"create_icon":           createIcon,
		"create_modal_target":   createModalTarget,
		"column_headers":        columnHeaders,
	}
}

// CrudListViewWithCreateLink is like CrudListView but opens createHref in the same tab instead of a modal.
// Pass createModalTarget as "" when using createHref.
func CrudListViewWithCreateLink(prefix, cardTitle, createLabel, createIcon, createHref string, columnHeaders []gin.H) gin.H {
	h := CrudListView(prefix, cardTitle, createLabel, createIcon, "", columnHeaders)
	h["create_href"] = createHref
	return h
}

// CrudListViewReadOnly is like CrudListView with no create action and no row-selection column
// (for audit / log viewers).
func CrudListViewReadOnly(prefix, cardTitle string, columnHeaders []gin.H) gin.H {
	h := CrudListView(prefix, cardTitle, "", "", "", columnHeaders)
	h["hide_row_select"] = true
	return h
}
