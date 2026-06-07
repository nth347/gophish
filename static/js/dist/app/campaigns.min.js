// labels is a map of campaign statuses to CSS classes
var labels = {
    "In progress": "label-primary",
    "Queued": "label-info",
    "Completed": "label-success",
    "Emails Sent": "label-success",
    "Error": "label-danger",
    "Todo": "label-warning"
}

var campaigns = []
var plans = []
var campaign = {}
var editingCampaignId = null
var editingStatus = null

function buildCampaignPayload() {
    var groups = []
    $("#users").select2("data").forEach(function (group) {
        groups.push({ name: group.text })
    })
    var send_by_date = $("#send_by_date").val()
    if (send_by_date != "") {
        send_by_date = moment(send_by_date, "MMMM Do YYYY, h:mm a").utc().format()
    }
    var webhookId = parseInt($("#webhook").val(), 10) || 0
    return {
        name: $("#name").val(),
        template: {
            name: $("#template").select2("data")[0] ? $("#template").select2("data")[0].text : ""
        },
        url: $("#url").val(),
        encryption_key: $("#encryption_key").val().trim(),
        page: { name: "" },
        smtp: {
            name: $("#profile").select2("data")[0] ? $("#profile").select2("data")[0].text : ""
        },
        launch_date: moment($("#launch_date").val(), "MMMM Do YYYY, h:mm a").utc().format(),
        send_by_date: send_by_date || null,
        groups: groups,
        webhook_id: webhookId
    }
}

// editCampaign opens the campaign modal pre-populated with the given campaign's data.
function editCampaign(id, status) {
    editingCampaignId = id
    editingStatus = status
    setupOptions()
    $("#campaignModalLabel").text("Edit Campaign")
    // Adjust footer: for Todo show both buttons; for others only "Save Changes"
    if (status === 'Todo') {
        $("#saveButton").html('<i class="fa fa-save"></i> Save Changes').show()
        $("#launchButton").html('<i class="fa fa-rocket"></i> Launch Campaign').show()
    } else {
        $("#saveButton").html('<i class="fa fa-save"></i> Save Changes').show()
        $("#launchButton").hide()
    }
    api.campaignId.get(id)
        .success(function (c) {
            $("#name").val(c.name)
            if (c.template && c.template.id) {
                $("#template").val(c.template.id.toString()).trigger("change.select2")
            } else if (c.template && c.template.name) {
                $("#template").select2({ placeholder: c.template.name })
            }
            if (c.smtp && c.smtp.id) {
                $("#profile").val(c.smtp.id.toString()).trigger("change.select2")
            } else if (c.smtp && c.smtp.name) {
                $("#profile").select2({ placeholder: c.smtp.name })
            }
            if (c.groups && c.groups.length > 0) {
                var groupIds = c.groups.map(function (g) { return g.id.toString() })
                $("#users").val(groupIds).trigger("change")
            }
            $("#url").val(c.url)
            $("#encryption_key").val(c.encryption_key)
            if (c.launch_date && c.launch_date !== "0001-01-01T00:00:00Z") {
                try { $("#launch_date").data("DateTimePicker").date(moment.utc(c.launch_date).local()) } catch (e) {}
            }
            if (c.send_by_date && c.send_by_date !== "0001-01-01T00:00:00Z") {
                try { $("#send_by_date").data("DateTimePicker").date(moment.utc(c.send_by_date).local()) } catch (e) {}
            }
            if (c.webhook_id) {
                $("#webhook").val(c.webhook_id.toString()).trigger("change")
            }
        })
        .error(function (data) {
            modalError(data.responseJSON.message)
        })
    $("#modal").modal({ backdrop: "static", keyboard: false })
    $("#modal").modal("show")
}

// updateCampaign PUTs the current form data to update the campaign.
function updateCampaign() {
    var payload = buildCampaignPayload()
    api.campaignId.put(editingCampaignId, payload)
        .success(function () {
            dismiss()
            Swal.fire('Updated!', 'Campaign has been updated successfully.', 'success')
                .then(function () { location.reload() })
        })
        .error(function (data) {
            modalError(data.responseJSON.message)
        })
}

// updateAndLaunch saves edits to a Todo campaign and then launches it.
function updateAndLaunch() {
    var payload = buildCampaignPayload()
    var id = editingCampaignId
    Swal.fire({
        title: "Save & Launch?",
        text: "Save changes and launch this campaign immediately?",
        type: "question",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Save & Launch",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.put(id, payload)
                    .success(function () {
                        api.campaignId.launch(id)
                            .success(function (data) {
                                campaign = data
                                resolve()
                            })
                            .error(function (data) {
                                reject(data.responseJSON.message)
                            })
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value) {
            dismiss()
            Swal.fire('Launched!', 'The campaign is now running.', 'success')
                .then(function () { location.reload() })
        }
    })
}

// launch POSTs the campaign and starts it immediately (or delegates to updateAndLaunch in edit mode)
function launch() {
    if (editingCampaignId !== null) {
        updateAndLaunch()
        return
    }
    Swal.fire({
        title: "Are you sure?",
        text: "This will schedule the campaign to be launched.",
        type: "question",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Launch",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                campaign = buildCampaignPayload()
                api.campaigns.post(campaign)
                    .success(function (data) {
                        resolve()
                        campaign = data
                    })
                    .error(function (data) {
                        $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>")
                        Swal.close()
                    })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('Campaign Scheduled!', 'This campaign has been scheduled for launch!', 'success')
        }
        $('button:contains("OK")').on('click', function () {
            window.location = "/campaigns/" + campaign.id.toString()
        })
    })
}

// savePlan POSTs the campaign with status "Todo" (saved, no emails sent), or delegates to updateCampaign in edit mode
function savePlan() {
    if (editingCampaignId !== null) {
        updateCampaign()
        return
    }
    var payload = buildCampaignPayload()
    payload.status = "Todo"
    api.campaigns.post(payload)
        .success(function (data) {
            campaign = data
            dismiss()
            Swal.fire(
                'Campaign Saved!',
                'Your campaign has been saved. Launch it whenever you\'re ready.',
                'success'
            ).then(function () {
                location.reload()
            })
        })
        .error(function (data) {
            $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>")
        })
}

// launchPlan launches a saved Todo campaign immediately
function launchPlan(id, name) {
    Swal.fire({
        title: "Launch Campaign?",
        text: "This will immediately start sending emails for \"" + name + "\".",
        type: "question",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Launch Now",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.launch(id)
                    .success(function (data) {
                        campaign = data
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('Campaign Launched!', 'The campaign is now running.', 'success')
        }
        $('button:contains("OK")').on('click', function () {
            window.location = "/campaigns/" + campaign.id.toString()
        })
    })
}

// Attempts to send a test email by POSTing to /util/send_test_email
function sendTestEmail() {
    var test_email_request = {
        template: {
            name: $("#template").select2("data")[0] ? $("#template").select2("data")[0].text : ""
        },
        first_name: $("input[name=to_first_name]").val(),
        last_name: $("input[name=to_last_name]").val(),
        email: $("input[name=to_email]").val(),
        position: $("input[name=to_position]").val(),
        url: $("#url").val(),
        encryption_key: $("#encryption_key").val().trim(),
        page: { name: "" },
        smtp: {
            name: $("#profile").select2("data")[0] ? $("#profile").select2("data")[0].text : ""
        }
    }
    btnHtml = $("#sendTestModalSubmit").html()
    $("#sendTestModalSubmit").html('<i class="fa fa-spinner fa-spin"></i> Sending')
    api.send_test_email(test_email_request)
        .success(function (data) {
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-success\">\
            <i class=\"fa fa-check-circle\"></i> Email Sent!</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
        .error(function (data) {
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
}

function dismiss() {
    $("#modal\\.flashes").empty()
    $("#name").val("")
    $("#template").val("").change()
    $("#page").val("").change()
    $("#url").val("")
    $("#encryption_key").val("")
    $("#profile").val("").change()
    $("#users").val("").change()
    $("#webhook").val(null).trigger("change")
    // Reset edit state
    editingCampaignId = null
    editingStatus = null
    $("#campaignModalLabel").text("New Campaign")
    $("#saveButton").html('<i class="fa fa-save"></i> Save as Plan').show()
    $("#launchButton").html('<i class="fa fa-rocket"></i> Launch Campaign').show()
    $("#modal").modal('hide')
}

function deleteCampaign(idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the campaign. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + campaigns[idx].name,
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.delete(campaigns[idx].id)
                    .success(function (msg) { resolve() })
                    .error(function (data) { reject(data.responseJSON.message) })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('Campaign Deleted!', 'This campaign has been deleted!', 'success')
        }
        $('button:contains("OK")').on('click', function () { location.reload() })
    })
}

function deletePlan(id) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the saved campaign. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.delete(id)
                    .success(function (msg) { resolve() })
                    .error(function (data) { reject(data.responseJSON.message) })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('Deleted!', 'The campaign has been deleted.', 'success')
        }
        $('button:contains("OK")').on('click', function () { location.reload() })
    })
}

function setupOptions() {
    api.groups.summary()
        .success(function (summaries) {
            groups = summaries.groups
            if (groups.length == 0) {
                modalError("No groups found!")
                return false
            } else {
                var group_s2 = $.map(groups, function (obj) {
                    obj.text = obj.name
                    obj.title = obj.num_targets + " targets"
                    return obj
                })
                $("#users.form-control").select2({
                    placeholder: "Select Groups",
                    data: group_s2
                })
            }
        })
    api.templates.get()
        .success(function (templates) {
            if (templates.length == 0) {
                modalError("No templates found!")
                return false
            } else {
                var template_s2 = $.map(templates, function (obj) {
                    obj.text = obj.name
                    return obj
                })
                var template_select = $("#template.form-control")
                template_select.select2({
                    placeholder: "Select a Template",
                    data: template_s2
                })
                if (templates.length === 1) {
                    template_select.val(template_s2[0].id)
                    template_select.trigger('change.select2')
                }
            }
        })
    api.SMTP.get()
        .success(function (profiles) {
            if (profiles.length == 0) {
                modalError("No profiles found!")
                return false
            } else {
                var profile_s2 = $.map(profiles, function (obj) {
                    obj.text = obj.name
                    return obj
                })
                var profile_select = $("#profile.form-control")
                profile_select.select2({
                    placeholder: "Select a Sending Profile",
                    data: profile_s2
                }).select2("val", profile_s2[0])
                if (profiles.length === 1) {
                    profile_select.val(profile_s2[0].id)
                    profile_select.trigger('change.select2')
                }
            }
        })
    api.webhooks.get()
        .success(function (webhooks) {
            var data = []
            if (webhooks && webhooks.length > 0) {
                $.each(webhooks, function (i, wh) {
                    if (wh.is_active) {
                        data.push({ id: wh.id, text: wh.name + " (" + wh.type + ")" })
                    }
                })
            }
            $("#webhook.form-control").select2({
                placeholder: "Use global webhooks",
                allowClear: true,
                data: data
            })
        })
}

function edit(campaign) {
    setupOptions()
}

function copy(idx) {
    setupOptions()
    api.campaignId.get(campaigns[idx].id)
        .success(function (campaign) {
            $("#name").val("Copy of " + campaign.name)
            if (!campaign.template.id) {
                $("#template").val("").change()
                $("#template").select2({ placeholder: campaign.template.name })
            } else {
                $("#template").val(campaign.template.id.toString())
                $("#template").trigger("change.select2")
            }
            if (!campaign.page.id) {
                $("#page").val("").change()
                $("#page").select2({ placeholder: campaign.page.name })
            } else {
                $("#page").val(campaign.page.id.toString())
                $("#page").trigger("change.select2")
            }
            if (!campaign.smtp.id) {
                $("#profile").val("").change()
                $("#profile").select2({ placeholder: campaign.smtp.name })
            } else {
                $("#profile").val(campaign.smtp.id.toString())
                $("#profile").trigger("change.select2")
            }
            $("#url").val(campaign.url)
            $("#encryption_key").val(campaign.encryption_key)
            if (campaign.webhook_id) {
                $("#webhook").val(campaign.webhook_id)
            }
        })
        .error(function (data) {
            $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>")
        })
}

$(document).ready(function () {
    $("#launch_date").datetimepicker({
        "widgetPositioning": { "vertical": "bottom" },
        "showTodayButton": true,
        "defaultDate": moment(),
        "format": "MMMM Do YYYY, h:mm a"
    })
    $("#send_by_date").datetimepicker({
        "widgetPositioning": { "vertical": "bottom" },
        "showTodayButton": true,
        "useCurrent": false,
        "format": "MMMM Do YYYY, h:mm a"
    })
    // Setup multiple modals
    $('.modal').on('hidden.bs.modal', function (event) {
        $(this).removeClass('fv-modal-stack')
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') - 1)
    })
    $('.modal').on('shown.bs.modal', function (event) {
        if (typeof ($('body').data('fv_open_modals')) == 'undefined') {
            $('body').data('fv_open_modals', 0)
        }
        if ($(this).hasClass('fv-modal-stack')) { return }
        $(this).addClass('fv-modal-stack')
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') + 1)
        $(this).css('z-index', 1040 + (10 * $('body').data('fv_open_modals')))
        $('.modal-backdrop').not('.fv-modal-stack').css('z-index', 1039 + (10 * $('body').data('fv_open_modals')))
        $('.modal-backdrop').not('fv-modal-stack').addClass('fv-modal-stack')
    })
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open')
    })
    $('#modal').on('hidden.bs.modal', function (event) {
        dismiss()
    })

    var activeCampaignsTable = $("#campaignTable").DataTable({
        columnDefs: [{ orderable: false, targets: "no-sort" }],
        order: [[1, "desc"]]
    })
    var archivedCampaignsTable = $("#campaignTableArchive").DataTable({
        columnDefs: [{ orderable: false, targets: "no-sort" }],
        order: [[1, "desc"]]
    })

    api.campaigns.summary()
        .success(function (data) {
            campaigns = data.campaigns
            $("#loading").hide()
            if (campaigns.length > 0) {
                $("#campaignTable").show()
                $("#campaignTableArchive").show()
                var activeRows = []
                var archivedRows = []
                $.each(campaigns, function (i, campaign) {
                    var label = labels[campaign.status] || "label-default"
                    var safeStatus = escapeHtml(campaign.status).replace(/"/g, '&quot;')
                    var actions
                    if (campaign.status === 'Todo') {
                        actions = "<div class='pull-right'>" +
                            "<button class='btn btn-default' onclick='editCampaign(" + campaign.id + ", \"" + safeStatus + "\")' data-toggle='tooltip' data-placement='left' title='Edit'>" +
                            "<i class='fa fa-pencil'></i></button> " +
                            "<button class='btn btn-primary' onclick='launchPlan(" + campaign.id + ", \"" + escapeHtml(campaign.name).replace(/"/g, '&quot;') + "\")' data-toggle='tooltip' data-placement='left' title='Launch Campaign'>" +
                            "<i class='fa fa-rocket'></i></button> " +
                            "<button class='btn btn-danger' onclick='deletePlan(" + campaign.id + ")' data-toggle='tooltip' data-placement='left' title='Delete'>" +
                            "<i class='fa fa-trash-o'></i></button></div>"
                    } else {
                        actions = "<div class='pull-right'><a class='btn btn-primary' href='/campaigns/" + campaign.id + "' data-toggle='tooltip' data-placement='left' title='View Results'>" +
                            "<i class='fa fa-bar-chart'></i></a> " +
                            "<button class='btn btn-default' onclick='editCampaign(" + campaign.id + ", \"" + safeStatus + "\")' data-toggle='tooltip' data-placement='left' title='View/Edit'>" +
                            "<i class='fa fa-pencil'></i></button> " +
                            "<span data-toggle='modal' data-backdrop='static' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Campaign' onclick='copy(" + i + ")'>" +
                            "<i class='fa fa-copy'></i></button></span> " +
                            "<button class='btn btn-danger' onclick='deleteCampaign(" + i + ")' data-toggle='tooltip' data-placement='left' title='Delete Campaign'>" +
                            "<i class='fa fa-trash-o'></i></button></div>"
                    }
                    var quickStats
                    if (campaign.status === 'Todo') {
                        quickStats = "Saved — not yet launched"
                    } else if (moment(campaign.launch_date).isAfter(moment())) {
                        quickStats = "Scheduled to start: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a') + "<br><br>Number of recipients: " + campaign.stats.total
                    } else {
                        quickStats = "Launch Date: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a') + "<br><br>Number of recipients: " + campaign.stats.total + "<br><br>Emails opened: " + campaign.stats.opened + "<br><br>Emails clicked: " + campaign.stats.clicked + "<br><br>Submitted Credentials: " + campaign.stats.submitted_data + "<br><br>Errors: " + campaign.stats.error + "<br><br>Reported: " + campaign.stats.email_reported
                    }
                    var row = [
                        escapeHtml(campaign.name),
                        moment(campaign.created_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<span class=\"label " + label + "\" data-toggle=\"tooltip\" data-placement=\"right\" data-html=\"true\" title=\"" + quickStats + "\">" + campaign.status + "</span>",
                        actions
                    ]
                    if (campaign.status === 'Completed') {
                        archivedRows.push(row)
                    } else {
                        activeRows.push(row)
                    }
                })
                activeCampaignsTable.rows.add(activeRows).draw()
                archivedCampaignsTable.rows.add(archivedRows).draw()
                $('[data-toggle="tooltip"]').tooltip()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching campaigns")
        })

    // Select2 Defaults
    $.fn.select2.defaults.set("width", "100%")
    $.fn.select2.defaults.set("dropdownParent", $("#modal_body"))
    $.fn.select2.defaults.set("theme", "bootstrap")
    $.fn.select2.defaults.set("sorter", function (data) {
        return data.sort(function (a, b) {
            if (a.text.toLowerCase() > b.text.toLowerCase()) return 1
            if (a.text.toLowerCase() < b.text.toLowerCase()) return -1
            return 0
        })
    })
})
