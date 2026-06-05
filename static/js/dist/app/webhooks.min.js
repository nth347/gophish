let webhooks = [];

const WEBHOOK_EVENTS = ["sent", "opened", "clicked", "submitted", "reported"];

// toggleWebhookType shows the Standard or Telegram fields based on the selected
// webhook type.
const toggleWebhookType = () => {
    if ($("#type").val() === "telegram") {
        $("#standard_fields").hide();
        $("#telegram_fields").show();
    } else {
        $("#telegram_fields").hide();
        $("#standard_fields").show();
    }
};

// getSelectedEvents returns the checked event keys as a comma-separated string.
const getSelectedEvents = () => {
    return WEBHOOK_EVENTS.filter((ev) => $("#event_" + ev).is(":checked")).join(",");
};

// setSelectedEvents checks the boxes for the given comma-separated event string.
// An empty string represents the legacy "all events" behavior, so all boxes are
// checked to reflect that.
const setSelectedEvents = (events) => {
    const all = !events || events.trim() === "";
    const list = all ? WEBHOOK_EVENTS : events.split(",").map((e) => e.trim());
    WEBHOOK_EVENTS.forEach((ev) => {
        $("#event_" + ev).prop("checked", all || list.indexOf(ev) !== -1);
    });
};

const dismiss = () => {
    $("#name").val("");
    $("#type").val("standard");
    $("#url").val("");
    $("#secret").val("");
    $("#telegram_bot_token").val("");
    $("#telegram_chat_id").val("");
    // Default new webhooks to notifying only on Submitted Data
    setSelectedEvents("submitted");
    $("#is_active").prop("checked", false);
    toggleWebhookType();
    $("#flashes").empty();
};

const saveWebhook = (id) => {
    let wh = {
        name: $("#name").val(),
        type: $("#type").val(),
        url: $("#url").val(),
        secret: $("#secret").val(),
        telegram_bot_token: $("#telegram_bot_token").val(),
        telegram_chat_id: $("#telegram_chat_id").val(),
        events: getSelectedEvents(),
        is_active: $("#is_active").is(":checked"),
    };
    if (id != -1) {
        wh.id = parseInt(id);
        api.webhookId.put(wh)
            .success(function(data) {
                dismiss();
                load();
                $("#modal").modal("hide");
                successFlash(`Webhook "${escapeHtml(wh.name)}" has been updated successfully!`);
            })
            .error(function(data) {
                modalError(data.responseJSON.message)
            })
    } else {
        api.webhooks.post(wh)
            .success(function(data) {
                load();
                dismiss();
                $("#modal").modal("hide");
                successFlash(`Webhook "${escapeHtml(wh.name)}" has been created successfully!`);
            })
            .error(function(data) {
                modalError(data.responseJSON.message)
            })
    }
};

const load = () => {
    $("#webhookTable").hide();
    $("#loading").show();
    api.webhooks.get()
        .success((whs) => {
            webhooks = whs;
            $("#loading").hide()
            $("#webhookTable").show()
            let webhookTable = $("#webhookTable").DataTable({
                destroy: true,
                columnDefs: [{
                    orderable: false,
                    targets: "no-sort"
                }]
            });
            webhookTable.clear();
            $.each(webhooks, (i, webhook) => {
                webhookTable.row.add([
                    escapeHtml(webhook.name),
                    escapeHtml(webhook.url),
                    escapeHtml(webhook.is_active),
                    `
                      <div class="pull-right">
                        <button class="btn btn-primary ping_button" data-webhook-id="${webhook.id}">
                          Ping
                        </button>
                        <button class="btn btn-primary edit_button" data-toggle="modal" data-backdrop="static" data-target="#modal" data-webhook-id="${webhook.id}">
                          <i class="fa fa-pencil"></i>
                        </button>
                        <button class="btn btn-danger delete_button" data-webhook-id="${webhook.id}">
                          <i class="fa fa-trash-o"></i>
                        </button>
                      </div>
                    `
                ]).draw()
            })
        })
        .error(() => {
            errorFlash("Error fetching webhooks")
        })
};

const editWebhook = (id) => {
    $("#modalSubmit").unbind("click").click(() => {
        saveWebhook(id);
    });
    if (id !== -1) {
        $("#webhookModalLabel").text("Edit Webhook")
        api.webhookId.get(id)
          .success(function(wh) {
              $("#name").val(wh.name);
              $("#type").val(wh.type || "standard");
              $("#url").val(wh.url);
              $("#secret").val(wh.secret);
              $("#telegram_bot_token").val(wh.telegram_bot_token);
              $("#telegram_chat_id").val(wh.telegram_chat_id);
              setSelectedEvents(wh.events);
              $("#is_active").prop("checked", wh.is_active);
              toggleWebhookType();
          })
          .error(function () {
              errorFlash("Error fetching webhook")
          });
    } else {
        // Reset the form to defaults (standard type, Submitted Data only)
        dismiss();
        $("#webhookModalLabel").text("New Webhook")
    }
};

const deleteWebhook = (id) => {
    var wh = webhooks.find(x => x.id == id);
    if (!wh) {
        return;
    }
    Swal.fire({
        title: "Are you sure?",
        text: `This will delete the webhook '${escapeHtml(wh.name)}'`,
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise((resolve, reject) => {
                api.webhookId.delete(id)
                    .success((msg) => {
                        resolve()
                    })
                    .error((data) => {
                        reject(data.responseJSON.message)
                    })
            })
            .catch(error => {
                Swal.showValidationMessage(error)
              })
        }
    }).then(function(result) {
        if (result.value) {
            Swal.fire(
                "Webhook Deleted!",
                `The webhook has been deleted!`,
                "success"
            );
        }
        $("button:contains('OK')").on("click", function() {
            location.reload();
        })
    })
};

const pingUrl = (btn, whId) => {
    dismiss();
    btn.disabled = true;
    api.webhookId.ping(whId)
        .success(function(wh) {
            btn.disabled = false;
            successFlash(`Ping of "${escapeHtml(wh.name)}" webhook succeeded.`);
        })
        .error(function(data) {
            btn.disabled = false;
            var wh = webhooks.find(x => x.id == whId);
            if (!wh) {
                return
            }
            errorFlash(`Ping of "${escapeHtml(wh.name)}" webhook failed: "${escapeHtml(data.responseJSON.message)}"`)
        });
};

$(document).ready(function() {
    load();
    $("#modal").on("hide.bs.modal", function() {
        dismiss();
    });
    $("#new_button").on("click", function() {
        editWebhook(-1);
    });
    $("#type").on("change", function() {
        toggleWebhookType();
    });
    $("#webhookTable").on("click", ".edit_button", function(e) {
        editWebhook($(this).attr("data-webhook-id"));
    });
    $("#webhookTable").on("click", ".delete_button", function(e) {
        deleteWebhook($(this).attr("data-webhook-id"));
    });
    $("#webhookTable").on("click", ".ping_button", function(e) {
        pingUrl(e.currentTarget, e.currentTarget.dataset.webhookId);
    });
});
