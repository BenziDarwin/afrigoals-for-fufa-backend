export type CustomerRefundRequest = {
 id:string;orderId:string;orderNumber:string;customerName:string;customerEmail:string;
 amount:number;kind:string;reason:string;details:string;receiptReference:string;
 originalTransactionId:string;maskedPhone:string;status:string;reviewNote:string;
 refundStatus:string;approvedAmount:number;refundTransactionId:string;createdAt:string;
};

export function refundRequestLabel(request:Pick<CustomerRefundRequest,"status"|"refundStatus">) {
 if(request.status==="declined")return "Request declined";
 if(request.status!=="approved")return "Awaiting admin review";
 switch(request.refundStatus){
  case "succeeded":return "Refund paid";
  case "failed":return "Approved · payment needs attention";
  case "rejected":return "Approved · refund draft withdrawn";
  case "draft":return "Approved · awaiting refund submission";
  default:return "Approved · awaiting refund confirmation";
 }
}
