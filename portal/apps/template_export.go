package apps

import (
	"cloudiac/portal/consts"
	"cloudiac/portal/consts/e"
	"cloudiac/portal/libs/ctx"
	"cloudiac/portal/libs/db"
	"cloudiac/portal/models"
	"cloudiac/portal/models/forms"
	"cloudiac/portal/services"
)

type TplExportForm struct {
	forms.BaseForm

	Ids      []models.Id `json:"ids" form:"ids" binding:required` // 待导出的云模板 id 列表
	Download bool        `json:"download" form:"download"`        // download 模式(直接返回导出数据 ，并触发浏览器下载)
}

func TemplateExport(c *ctx.ServiceContext, form *TplExportForm) (interface{}, e.Error) {
	return services.ExportTemplates(c.DB(), c.OrgId, form.Ids)
}

type TplImportForm struct {
	forms.BaseForm

	IdDuplicate string      `json:"idDuplicate" form:"idDuplicate" bind:"required"` // id 重复时的处理方式, enum('update','skip','copy','abort')
	Projects    []models.Id `json:"projects" form:"projects"`                       // 关联项目 id 列表

	Data services.TplExportedData `json:"data" form:"file" bind:"required"` // 待导入数据(即导出的数据)
}

func TemplateImport(c *ctx.ServiceContext, form *TplImportForm) (result *services.TplImportResult, er e.Error) {
	importer := services.TplImporter{
		Logger:          c.Logger().WithField("action", "ImportTemplate"),
		OrgId:           c.OrgId,
		CreatorId:       consts.SysUserId,
		ProjectIds:      form.Projects,
		Data:            form.Data,
		WhenIdDuplicate: form.IdDuplicate,
	}
	// return importer.Import(c.DB())

	err := c.DB().Transaction(func(tx *db.Session) error {
		result, er = importer.Import(tx)
		return er
	})
	if err != nil {
		return nil, e.AutoNew(err, e.DBError)
	}
	return result, nil
}
