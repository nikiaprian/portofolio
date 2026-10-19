package handler

import "fmt"

// siteLayoutSharedRuntimeRootVars are root template keys injected on most client pages.
var siteLayoutSharedRuntimeRootVars = []string{
	"title",
	"meta_title",
	"meta_description",
	"meta_keywords",
	"meta_image",
	"publisher_image",
	"og_type",
	"site_name",
	"footer_copy",
	"logo_url",
	"favicon_url",
	"tagline",
	"site_description",
	"theme_color",
	"accent_color",
	"background_primary",
	"background_secondary",
	"text_primary",
	"text_secondary",
	"border_color",
	"font_family",
	"content_font_size",
	"sidebar_font_size",
	"image_caption_enabled",
	"image_caption_mode",
	"image_caption_template",
	"also_read_enabled",
	"also_read_max_inserts",
	"also_read_paragraph_interval",
	"image_resize_enabled",
	"img_resize_enabled",
	"img_resize_width",
	"img_resize_height",
	"img_resize_quality",
	"image_resize_width",
	"image_resize_quality",
	"ads_lazy_load_enabled",
	"avatar_resize_width",
	"avatar_resize_quality",
	"homepage_layout",
	"static_version",
	"header_script",
	"footer_script",
	"ads_top",
	"ads_center",
	"ads_bottom",
	"ads_sidebar",
	"social_facebook_url",
	"social_x_url",
	"social_instagram_url",
	"social_youtube_url",
	"social_tiktok_url",
	"social_rss_url",
	"current_url",
	"public_base",
	"client_pages",
	"nav_categories",
	"load_app_js",
	"load_post_js",
	"local_timezone",
	"login_href",
	"popular_posts",
	"latest_posts",
	"sidebar_list_limit",
	"sidebar_list_limit_popular",
	"sidebar_list_limit_latest",
	"current_year",
}

// siteLayoutFragmentRuntimeRootVars are extra root keys provided for specific fragments.
var siteLayoutFragmentRuntimeRootVars = map[string][]string{
	"home_body": {
		"home_ssr",
		"home_slides",
		"home_tags",
		"home_feed",
		"home_feed_total_pages",
		"home_latest",
		"is_homepage",
		"detik_recommend_posts",
		"neo_ticker_posts",
		"neo_featured_tabs",
		"home_sub_featured",
		"home_selected_posts",
		"home_hero_side",
		"home_informasi_posts",
		"home_category_strip_name",
		"home_category_strip_href",
		"home_category_strip_posts",
		"home_theme3_lead",
		"home_theme3_more",
		"home_theme3_stack",
		"home_theme3_wire",
		"home_theme3_feed",
		"home_theme3_aside",
		"ads_sidebar",
		"date_label",
		"rank",
		"author_name",
		"time",
		"has_cat",
		"cat_name",
		"cat_upper",
		"thumb",
		"title",
		"href",
		"rel_time",
		"breaking",
		"excerpt",
	},
	"article_body": {
		"post_slug",
		"post_title",
		"post_body",
		"cat_name",
		"cat_slug",
		"cat_href",
		"post_time_label",
		"post_read_label",
		"author_name",
		"author_img",
		"avatar_letter",
		"author_path",
		"author_bio",
		"hero_url",
		"hero_caption_html",
		"post_source",
		"meta_author",
		"article_schema_type",
		"post_created_at_iso",
		"post_updated_at_iso",
	},
	"sidebar": {
		"cat_name",
		"has_cat",
		"rank",
		"author_name",
		"time",
		"date_label",
		"thumb",
		"title",
		"href",
	},
	"rss": {
		"rss_rows",
	},
	"author_profile": {
		"author_img",
		"author_name",
		"author_role_label",
		"author_bio",
		"author_avatar_letter",
		"author_posts_count",
		"posts",
		"page",
		"total_pages",
	},
	"tag": {
		"tag_name",
		"posts",
		"page",
		"total_pages",
	},
	"kategori": {
		"category_name",
		"category_parent_name",
		"category_parent_href",
		"posts",
		"page",
		"total_pages",
	},
	"search_page": {
		"posts",
		"page",
		"total_pages",
	},
}

func siteLayoutAllowedRootVars(fragment, existingFile string) (map[string]struct{}, error) {
	allowed := make(map[string]struct{})

	existingVars, err := siteLayoutCollectRootVars(existingFile)
	if err != nil {
		return nil, fmt.Errorf("%w: template referensi: %v", errSiteLayoutApplyInvalid, err)
	}
	for v := range existingVars {
		allowed[v] = struct{}{}
	}

	for _, name := range siteLayoutSharedRuntimeRootVars {
		allowed[name] = struct{}{}
	}
	for _, name := range siteLayoutFragmentRuntimeRootVars[fragment] {
		allowed[name] = struct{}{}
	}

	return allowed, nil
}
