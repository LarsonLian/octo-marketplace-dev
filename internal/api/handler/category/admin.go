package category

import (
	"errors"
	"net/http"

	"github.com/Mininglamp-OSS/octo-marketplace/internal/api/errcode"
	apiresponse "github.com/Mininglamp-OSS/octo-marketplace/internal/api/response"
	"github.com/Mininglamp-OSS/octo-marketplace/internal/middleware"
	categorysvc "github.com/Mininglamp-OSS/octo-marketplace/internal/service/category"
	"github.com/gin-gonic/gin"
)

func (h *Handler) RegisterAdmin(rg *gin.RouterGroup, idGen func() string, authEnabled bool) {
	h.idGen = idGen
	admin := rg.Group("/admin/skill_categories", requireAdmin(authEnabled))
	admin.POST("", h.AdminCreate)
	admin.PATCH("/:skill_category_id", h.AdminUpdate)
	admin.DELETE("/:skill_category_id", h.AdminDelete)

	legacy := rg.Group("/skill/admin/categories", requireAdmin(authEnabled), legacyCategoryEndpoint)
	legacy.POST("", h.AdminCreate)
	legacy.PUT("/:skill_category_id", h.AdminUpdate)
	legacy.DELETE("/:skill_category_id", h.AdminDelete)
}

func requireAdmin(authEnabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !authEnabled {
			c.Next()
			return
		}
		identity, ok := middleware.Identity(c)
		if !ok {
			apiresponse.Fail(c, http.StatusUnauthorized, errcode.Unauthorized, "authentication is required", nil, "")
			return
		}
		if identity.Role != "admin" {
			apiresponse.Fail(c, http.StatusForbidden, errcode.PermissionDenied, "admin access is required", nil, "")
			return
		}
		c.Next()
	}
}

type AdminCategoryRequest struct {
	Name      string `json:"name" binding:"required"`
	IconKey   string `json:"icon_key"`
	SortOrder int    `json:"sort_order"`
}

// AdminCreate godoc
// @Summary Create Skill category
// @Description Create a Skill category through the authenticated administrator surface.
// @Tags skill_category
// @ID skill_category.create
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body AdminCategoryRequest true "Skill category"
// @Success 201 {object} apiresponse.Data[categorysvc.AdminItem]
// @Failure 400 {object} apiresponse.Error "VALIDATION_ERROR"
// @Failure 401 {object} apiresponse.Error "AUTH_REQUIRED"
// @Failure 403 {object} apiresponse.Error "FORBIDDEN"
// @Failure 404 {object} apiresponse.Error "NOT_FOUND"
// @Failure 500 {object} apiresponse.Error "INTERNAL_ERROR"
// @Router /admin/skill_categories [post]
func (h *Handler) AdminCreate(c *gin.Context) {
	if _, ok := middleware.Identity(c); !ok {
		apiresponse.Fail(c, http.StatusUnauthorized, errcode.Unauthorized, "authentication is required", nil, "")
		return
	}
	var req AdminCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.Fail(c, http.StatusBadRequest, errcode.BadRequest, "name is required", nil, "")
		return
	}
	item, err := h.svc.Create(c.Request.Context(), h.idGen(), req.Name, req.IconKey, req.SortOrder)
	if err != nil {
		apiresponse.Fail(c, http.StatusInternalServerError, errcode.InternalError, "internal error", nil, "")
		return
	}
	apiresponse.Created(c, item)
}

// AdminUpdate godoc
// @Summary Update Skill category
// @Description Replace mutable fields of an existing Skill category.
// @Tags skill_category
// @ID skill_category.update
// @Accept json
// @Produce json
// @Security Bearer
// @Param skill_category_id path string true "Skill category ID"
// @Param body body AdminCategoryRequest true "Skill category changes"
// @Success 200 {object} apiresponse.Data[categorysvc.AdminItem]
// @Failure 400 {object} apiresponse.Error "VALIDATION_ERROR"
// @Failure 401 {object} apiresponse.Error "AUTH_REQUIRED"
// @Failure 403 {object} apiresponse.Error "FORBIDDEN"
// @Failure 404 {object} apiresponse.Error "NOT_FOUND"
// @Failure 500 {object} apiresponse.Error "INTERNAL_ERROR"
// @Router /admin/skill_categories/{skill_category_id} [patch]
func (h *Handler) AdminUpdate(c *gin.Context) {
	if _, ok := middleware.Identity(c); !ok {
		apiresponse.Fail(c, http.StatusUnauthorized, errcode.Unauthorized, "authentication is required", nil, "")
		return
	}
	var req AdminCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.Fail(c, http.StatusBadRequest, errcode.BadRequest, "name is required", nil, "")
		return
	}
	item, err := h.svc.Update(c.Request.Context(), c.Param("skill_category_id"), req.Name, req.IconKey, req.SortOrder)
	if errors.Is(err, categorysvc.ErrCategoryNotFound) {
		apiresponse.Fail(c, http.StatusNotFound, errcode.NotFound, "category not found", nil, "")
		return
	}
	if err != nil {
		apiresponse.Fail(c, http.StatusInternalServerError, errcode.InternalError, "internal error", nil, "")
		return
	}
	apiresponse.OK(c, item)
}

// AdminDelete godoc
// @Summary Delete Skill category
// @Description Delete an unused Skill category; categories referenced by Skills are rejected.
// @Tags skill_category
// @ID skill_category.delete
// @Accept json
// @Produce json
// @Security Bearer
// @Param skill_category_id path string true "Skill category ID"
// @Success 200 {object} apiresponse.Data[apiresponse.EmptyResp]
// @Failure 401 {object} apiresponse.Error "AUTH_REQUIRED"
// @Failure 403 {object} apiresponse.Error "FORBIDDEN"
// @Failure 404 {object} apiresponse.Error "NOT_FOUND"
// @Failure 409 {object} apiresponse.Error "CONFLICT"
// @Failure 500 {object} apiresponse.Error "INTERNAL_ERROR"
// @Router /admin/skill_categories/{skill_category_id} [delete]
func (h *Handler) AdminDelete(c *gin.Context) {
	if _, ok := middleware.Identity(c); !ok {
		apiresponse.Fail(c, http.StatusUnauthorized, errcode.Unauthorized, "authentication is required", nil, "")
		return
	}
	count, err := h.svc.Delete(c.Request.Context(), c.Param("skill_category_id"))
	if errors.Is(err, categorysvc.ErrCategoryInUse) {
		apiresponse.Fail(c, http.StatusConflict, errcode.CategoryInUse, "category is in use", map[string]any{"skill_count": count}, "Move the Skills before deleting this category.")
		return
	}
	if errors.Is(err, categorysvc.ErrCategoryNotFound) {
		apiresponse.Fail(c, http.StatusNotFound, errcode.NotFound, "category not found", nil, "")
		return
	}
	if err != nil {
		apiresponse.Fail(c, http.StatusInternalServerError, errcode.InternalError, "internal error", nil, "")
		return
	}
	apiresponse.Empty(c)
}
