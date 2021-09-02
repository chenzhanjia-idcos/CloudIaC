package services

import (
	"cloudiac/portal/consts"
	"cloudiac/portal/consts/e"
	"cloudiac/portal/libs/db"
	"cloudiac/portal/models"
	"cloudiac/portal/models/forms"
	"cloudiac/runner"
	"fmt"
	"strconv"
	"strings"
)

func CreatePolicy(tx *db.Session, policy *models.Policy) (*models.Policy, e.Error) {
	if err := models.Create(tx, policy); err != nil {
		if e.IsDuplicate(err) {
			return nil, e.New(e.PolicyAlreadyExist, err)
		}
		return nil, e.New(e.DBError, err)
	}
	return policy, nil
}

// GetPolicyReferenceId 生成策略ID
// reference id = "iac" + policy type + creator scope + max id
func GetPolicyReferenceId(query *db.Session, policy *models.Policy) (string, e.Error) {
	typ := "iac"
	if policy.PolicyType != "" {
		typ = policy.PolicyType
	}
	lastId := 0
	// query max id by type
	po := models.Policy{}
	if err := query.Model(models.Policy{}).Where("reference_id LIKE ?", "iac_"+typ+"%").
		Order("length(reference_id) DESC, reference_id DESC").Last(&po); err != nil && !e.IsRecordNotFound(err) {
		return "", e.AutoNew(err, e.DBError)
	}
	idx := strings.LastIndex(po.ReferenceId, "_")
	if idx != -1 {
		lastId, _ = strconv.Atoi(po.ReferenceId[idx+1:])
	}

	// internal or public
	scope := "public"

	return fmt.Sprintf("%s_%s_%s_%d", "iac", typ, scope, lastId+1), nil
}

func GetPolicyById(tx *db.Session, id models.Id) (*models.Policy, e.Error) {
	po := models.Policy{}
	if err := tx.Model(models.Policy{}).Where("id = ?", id).First(&po); err != nil {
		if e.IsRecordNotFound(err) {
			return nil, e.New(e.PolicyNotExist, err)
		}
		return nil, e.New(e.DBError, err)
	}
	return &po, nil
}

func GetTaskPolicies(query *db.Session, taskId models.Id) ([]runner.TaskPolicy, e.Error) {
	var taskPolicies []runner.TaskPolicy
	policies, err := GetValidTaskPolicyIds(query, taskId)
	if err != nil {
		return nil, err
	}
	for _, policyId := range policies {
		policy, err := GetPolicyById(query, policyId)
		if err != nil {
			return nil, err
		}
		category := "general"
		group, _ := GetPolicyGroupById(query, policy.GroupId)
		if group != nil {
			category = group.Name
		}
		meta := map[string]interface{}{
			"name":          policy.Name,
			"file":          "policy.rego",
			"policy_type":   policy.PolicyType,
			"resource_type": policy.ResourceType,
			"severity":      strings.ToUpper(policy.Severity),
			"reference_id":  policy.ReferenceId,
			"category":      category,
			"version":       policy.Revision,
			"id":            string(policy.Id),
		}
		taskPolicies = append(taskPolicies, runner.TaskPolicy{
			PolicyId: string(policyId),
			Meta:     meta,
			Rego:     policy.Rego,
		})
	}
	return taskPolicies, nil
}

// GetValidTaskPolicyIds 获取策略关联的策略ID列表
func GetValidTaskPolicyIds(query *db.Session, taskId models.Id) ([]models.Id, e.Error) {
	// TODO: 处理策略关联及已经屏蔽策略

	var (
		policies  []models.Policy
		policyIds []models.Id
		envId     models.Id
		tplId     models.Id
		err       e.Error
	)
	if task, err := GetTask(query, taskId); err != nil {
		if e.IsRecordNotFound(err) {
			if scantask, err := GetScanTaskById(query, taskId); err != nil {
				return nil, err
			} else {
				envId = scantask.EnvId
				tplId = scantask.TplId
			}
		} else {
			return nil, err
		}
	} else {
		envId = task.EnvId
		tplId = task.TplId
	}

	if envId != "" {
		policies, err = GetPoliciesByEnvId(query, envId)
		if err != nil {
			return nil, err
		}
	} else {
		policies, err = GetPoliciesByTemplateId(query, tplId)
		if err != nil {
			return nil, err
		}
	}

	for _, policy := range policies {
		policyIds = append(policyIds, policy.Id)
	}

	return policyIds, nil
}

func GetPoliciesByEnvId(query *db.Session, envId models.Id) ([]models.Policy, e.Error) {
	subQuery := query.Table(models.PolicyRel{}.TableName()).Select("group_id").Where("env_id = ?", envId)
	var policies []models.Policy
	query = query.Debug()
	if err := query.Model(models.Policy{}).Where("group_id in (?)", subQuery.Expr()).Find(&policies); err != nil {
		if e.IsRecordNotFound(err) {
			return nil, e.New(e.PolicyNotExist, err)
		}
		return nil, e.New(e.DBError, err)
	}

	// TODO: 处理策略屏蔽策略关系
	return policies, nil
}

func GetPoliciesByTemplateId(query *db.Session, tplId models.Id) ([]models.Policy, e.Error) {
	subQuery := query.Table(models.PolicyRel{}.TableName()).Select("group_id").Where("tpl_id = ?", tplId)
	var policies []models.Policy
	if err := query.Model(models.Policy{}).Where("group_id in (?)", subQuery.Expr()).Find(&policies); err != nil {
		if e.IsRecordNotFound(err) {
			return nil, e.New(e.PolicyNotExist, err)
		}
		return nil, e.New(e.DBError, err)
	}

	// TODO: 处理策略屏蔽策略关系
	return policies, nil
}

func UpdatePolicy(tx *db.Session, policy *models.Policy, attr models.Attrs) (int64, e.Error) {
	affected, err := models.UpdateAttr(tx, policy, attr)
	if err != nil {
		if e.IsDuplicate(err) {
			return affected, e.New(e.PolicyGroupAlreadyExist, err)
		}
		return affected, e.New(e.DBError, err)
	}
	return affected, nil
}

//RemovePoliciesGroupRelation 移除策略组和策略的关系
func RemovePoliciesGroupRelation(tx *db.Session, groupId models.Id) e.Error {
	if _, err := UpdatePolicy(tx.Where("group_id = ?", groupId),
		&models.Policy{}, models.Attrs{"group_id": ""}); err != nil {
		return err
	}
	return nil
}

func SearchPolicy(dbSess *db.Session, form *forms.SearchPolicyForm) *db.Session {
	pTable := models.Policy{}.TableName()
	query := dbSess.Table(pTable)
	if len(form.GroupId) > 0 {
		query = query.Where(fmt.Sprintf("%s.group_id in (?)", pTable), form.GroupId)
	}

	if form.Severity != "" {
		query = query.Where(fmt.Sprintf("%s.severity = ?", pTable), form.Severity)
	}

	if form.Q != "" {
		qs := "%" + form.Q + "%"
		query = query.Where(fmt.Sprintf("%s.name like ?", pTable), qs)
	}

	query = query.Joins("left join iac_policy_group as g on g.id = iac_policy.group_id").
		LazySelectAppend("iac_policy.*,g.name as group_name")

	query = query.Joins("left join iac_user as u on u.id = iac_policy.creator_id").
		LazySelectAppend("iac_policy.*,u.name as creator")

	return query
}

func DeletePolicy(dbSess *db.Session, id models.Id) (interface{}, e.Error) {
	if _, err := dbSess.
		Where("id = ?", id).
		Delete(&models.Policy{}); err != nil {
		return nil, e.New(e.DBError, err)
	}

	return nil, nil
}

func DetailPolicy(dbSess *db.Session, id models.Id) (interface{}, e.Error) {
	p := models.Policy{}
	if err := dbSess.Table(models.Policy{}.TableName()).
		Where("id = ?", id).
		First(&p); err != nil {
		if e.IsRecordNotFound(err) {
			return nil, e.New(e.DBError, fmt.Errorf("polict not found id: %s", id))
		}
		return nil, e.New(e.DBError, err)
	}
	return nil, nil
}

func CreatePolicySuppress() (interface{}, e.Error) {
	return nil, nil
}

func SearchPolicySuppress() (interface{}, e.Error) {
	return nil, nil
}

func DeletePolicySuppress() (interface{}, e.Error) {
	return nil, nil
}

func SearchPolicyTpl() (interface{}, e.Error) {
	return nil, nil
}

func UpdatePolicyTpl() (interface{}, e.Error) {
	return nil, nil
}

func DetailPolicyTpl() (interface{}, e.Error) {
	return nil, nil
}

func SearchPolicyEnv() (interface{}, e.Error) {
	return nil, nil
}

func UpdatePolicyEnv() (interface{}, e.Error) {
	return nil, nil
}

func DetailPolicyEnv() (interface{}, e.Error) {
	return nil, nil
}

func PolicyError() (interface{}, e.Error) {
	return nil, nil
}

func PolicyReference() (interface{}, e.Error) {
	return nil, nil
}

func PolicyRepo() (interface{}, e.Error) {
	return nil, nil
}

type PolicyScanSummary struct {
	Id     models.Id `json:"id"`
	Count  int       `json:"count"`
	Status string    `json:"status"`
}

// PolicySummary 获取策略环境/云模板执行结果
func PolicySummary(query *db.Session, ids []models.Id, scope string) ([]*PolicyScanSummary, e.Error) {
	subQuery := query.Model(models.PolicyResult{}).Select("min(id)").Group("policy_id,env_id,tpl_id")
	if scope == consts.ScopePolicy {
		subQuery = subQuery.Where("policy_id in (?)", ids)
	} else {
		subQuery = subQuery.Where("policy_group_id in (?)", ids)
	}
	q := query.Model(models.PolicyResult{}).Select("policy_id as id,count(*) as count,status").
		Where("id in (?)", subQuery.Expr()).Group("policy_id,status")

	summary := make([]*PolicyScanSummary, 0)
	if err := q.Find(&summary); err != nil {
		if e.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, e.New(e.DBError, err)
	}

	return summary, nil
}
