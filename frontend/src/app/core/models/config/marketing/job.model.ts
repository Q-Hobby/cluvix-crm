import {UserProfileModel} from "@/core/models/user-profile.model";

export interface JobModel {
    name?: string;
    id?: number;
    status?: number;
    desc?: string;
    created_at?: number;
    updated_at?: number;
    created_by?: number;
    updated_by?: number;
    userCreated?: UserProfileModel;
    userUpdated?: UserProfileModel;
    SortOrder?: number;

}
